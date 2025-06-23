package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"

	userservice "postic-backend/internal/delivery/grpc/user-service"
	userserviceproto "postic-backend/internal/delivery/grpc/user-service/proto"
	"postic-backend/internal/delivery/http/utils"
	"postic-backend/internal/repo/cockroach"
	"postic-backend/internal/usecase/service"
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
	// Настройка контекста для graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()

	// Получаем переменные окружения
	dbConnectDSN := os.Getenv("DB_CONNECT_DSN")
	grpcPort := os.Getenv("USER_SERVICE_GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50051" // Порт по умолчанию
	}

	vkClientID := os.Getenv("VK_CLIENT_ID")
	vkClientSecret := os.Getenv("VK_CLIENT_SECRET")
	vkRedirectURL := os.Getenv("VK_REDIRECT_URL")
	//log.Infof("%s %s %s", vkClientID, vkClientSecret, vkRedirectURL)

	// Подключение к базе данных
	dbConn, err := connector.GetCockroachConnector(dbConnectDSN)
	if err != nil {
		log.Fatalf("Ошибка при подключении к базе данных: %v", err)
	}
	defer func() {
		if err := dbConn.Close(); err != nil {
			log.Errorf("Ошибка при закрытии соединения с базой данных: %v", err)
		}
	}()

	// Инициализация репозиториев
	userRepo := cockroach.NewUser(dbConn)

	// Инициализация VK OAuth
	vkAuth := utils.NewVKOAuth(vkClientID, vkClientSecret, vkRedirectURL)

	// Инициализация usecase
	userUseCase := service.NewUser(userRepo, vkAuth)

	// Создание gRPC сервера
	grpcServer := grpc.NewServer() // Создание и регистрация gRPC сервиса
	userServiceServer := userservice.NewUserServiceServer(userUseCase)
	userserviceproto.RegisterUserServiceServer(grpcServer, userServiceServer)

	// Запуск gRPC сервера
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		log.Fatalf("Ошибка при создании listener: %v", err)
	}

	log.Infof("User service запущен на порту %s", grpcPort)

	// Запуск сервера в горутине
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Errorf("Ошибка при запуске gRPC сервера: %v", err)
		}
	}()

	// Ожидание сигнала завершения
	<-ctx.Done()
	log.Info("Получен сигнал завершения, останавливаем сервер...")

	// Graceful shutdown
	grpcServer.GracefulStop()
	log.Info("User service успешно остановлен")
}
