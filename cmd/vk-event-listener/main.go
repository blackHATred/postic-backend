package main

import (
	"context"
	"os"
	"os/signal"
	uploadservice "postic-backend/internal/delivery/grpc/upload-service"
	"postic-backend/internal/repo/cockroach"
	"postic-backend/internal/repo/kafka"
	"postic-backend/internal/usecase/service"
	"postic-backend/internal/usecase/service/vkontakte"
	"postic-backend/pkg/connector"
	"postic-backend/pkg/goosehelper"
	"strings"

	"github.com/joho/godotenv"
	"github.com/labstack/gommon/log"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Info(".env файл не обнаружен")
	}

	// Выполнить миграции при старте
	dbConnectDSN := os.Getenv("DB_CONNECT_DSN")
	DBConn, err := connector.GetCockroachConnector(dbConnectDSN)
	if err != nil {
		log.Fatalf("Ошибка при подключении к базе данных: %v", err)
	}
	// Получаем *sql.DB из *sqlx.DB
	sqldb := DBConn.DB
	migrationsDir := "./cockroachdb/migrations"
	goosehelper.MigrateUp(sqldb, migrationsDir)
	if err := DBConn.Close(); err != nil {
		log.Fatalf("Ошибка при закрытии соединения с базой данных: %v", err)
	}
}

func main() {
	sysCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()

	dbConnectDSN := os.Getenv("DB_CONNECT_DSN")
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")

	uploadServiceAddr := os.Getenv("UPLOAD_SERVICE_ADDR")
	if uploadServiceAddr == "" {
		uploadServiceAddr = "localhost:50052"
	}

	DBConn, err := connector.GetCockroachConnector(dbConnectDSN)
	if err != nil {
		log.Fatalf("Ошибка при подключении к базе данных: %v", err)
	}
	defer func() {
		err := DBConn.Close()
		if err != nil {
			log.Fatalf("Ошибка при закрытии соединения с базой данных: %v", err)
		}
	}()

	eventRepo, err := kafka.NewCommentEventKafkaRepository(strings.Split(kafkaBrokers, ","))
	if err != nil {
		log.Fatalf("Ошибка при создании Kafka репозитория: %v", err)
	}
	teamRepo := cockroach.NewTeam(DBConn)
	postRepo := cockroach.NewPost(DBConn)
	commentRepo := cockroach.NewComment(DBConn)
	vkontakteListenerRepo := cockroach.NewVkontakteListener(DBConn)

	// gRPC upload client
	uploadClient, err := uploadservice.NewClient(uploadServiceAddr)
	if err != nil {
		log.Fatalf("Ошибка при создании gRPC клиента для upload service: %v", err)
	}
	defer uploadClient.Close()
	uploadUseCase := service.NewUpload(uploadClient)

	vkEventListener := vkontakte.NewVKEventListener(vkontakteListenerRepo, teamRepo, postRepo, uploadUseCase, commentRepo, eventRepo)
	go vkEventListener.StartListener()
	log.Infof("VK Event Listener запущен, слушаем события...")
	defer vkEventListener.StopListener()

	<-sysCtx.Done()
}
