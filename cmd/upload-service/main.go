package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"

	uploadservice "postic-backend/internal/delivery/grpc/upload-service"
	uploadpb "postic-backend/internal/delivery/grpc/upload-service/proto"
	"postic-backend/internal/repo/cockroach"
	"postic-backend/pkg/connector"
	"postic-backend/pkg/goosehelper"

	"github.com/joho/godotenv"
	"github.com/labstack/gommon/log"
	"google.golang.org/grpc"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Info(".env файл не обнаружен")
	}

	dbConnectDSN := os.Getenv("DB_CONNECT_DSN")
	DBConn, err := connector.GetCockroachConnector(dbConnectDSN)
	if err != nil {
		log.Fatalf("Ошибка при подключении к базе данных: %v", err)
	}
	sqldb := DBConn.DB
	migrationsDir := "./cockroachdb/migrations"
	goosehelper.MigrateUp(sqldb, migrationsDir)
	if err := DBConn.Close(); err != nil {
		log.Fatalf("Ошибка при закрытии соединения с базой данных: %v", err)
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()

	dbConnectDSN := os.Getenv("DB_CONNECT_DSN")
	grpcPort := os.Getenv("UPLOAD_SERVICE_GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50052"
	}

	dbConn, err := connector.GetCockroachConnector(dbConnectDSN)
	if err != nil {
		log.Fatalf("Ошибка при подключении к базе данных: %v", err)
	}
	defer func() {
		if err := dbConn.Close(); err != nil {
			log.Errorf("Ошибка при закрытии соединения с базой данных: %v", err)
		}
	}()

	minioEndpoint := os.Getenv("MINIO_ENDPOINT")
	minioAccessKey := os.Getenv("MINIO_ACCESS_KEY")
	minioSecretKey := os.Getenv("MINIO_SECRET_KEY")
	minioUseSSL := os.Getenv("MINIO_USE_SSL") == "true"

	minioClient, err := connector.GetMinioConnector(minioEndpoint, minioAccessKey, minioSecretKey, minioUseSSL)
	if err != nil {
		log.Fatalf("Ошибка при подключении к MinIO: %v", err)
	}

	uploadRepo, err := cockroach.NewUpload(dbConn, minioClient)
	if err != nil {
		log.Fatalf("Ошибка при инициализации uploadRepo: %v", err)
	}

	uploadServiceServer := uploadservice.NewUploadServiceServer(uploadRepo)
	grpcServer := grpc.NewServer()
	uploadpb.RegisterUploadServiceServer(grpcServer, uploadServiceServer)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		log.Fatalf("Ошибка при создании listener: %v", err)
	}

	log.Infof("Upload service запущен на порту %s", grpcPort)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Errorf("Ошибка при запуске gRPC сервера: %v", err)
		}
	}()

	<-ctx.Done()
	log.Info("Получен сигнал завершения, останавливаем сервер...")
	grpcServer.GracefulStop()
	log.Info("Upload service успешно остановлен")
}
