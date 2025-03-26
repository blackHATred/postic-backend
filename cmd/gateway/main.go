package main

import (
	"context"
	"errors"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"net/http"
	"os"
	"os/signal"
	delivery "postic-backend/internal/delivery/http"
	"postic-backend/internal/delivery/http/utils"
	"postic-backend/internal/repo/cockroach"
	"postic-backend/internal/usecase/service"
	"postic-backend/pkg/connector"
	"time"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Info(".env файл не обнаружен")
	}
	telegramBotToken := os.Getenv("TELEGRAM_BOT_TOKEN")

	// cockroach
	DBConn, err := connector.GetCockroachConnector("user=root dbname=defaultdb sslmode=disable port=26257") // примерный вид dsn: "user=root dbname=defaultdb sslmode=disable"
	if err != nil {
		log.Fatalf("Ошибка при подключении к базе данных: %v", err)
	}
	defer func() {
		err := DBConn.Close()
		if err != nil {
			log.Fatalf("Ошибка при закрытии соединения с базой данных: %v", err)
		}
	}()

	// minio
	minioClient, err := connector.GetMinioConnector("localhost:9000", "minioadmin", "minioadmin", false)
	if err != nil {
		log.Fatalf("Ошибка при подключении к MinIO: %v", err)
	}

	// запускаем сервисы репозиториев (подключение к базе данных)
	userRepo := cockroach.NewUser(DBConn)
	postRepo := cockroach.NewPost(DBConn)
	uploadRepo, err := cockroach.NewUpload(DBConn, minioClient)
	if err != nil {
		log.Fatalf("Ошибка при создании репозитория Upload: %v", err)
	}
	commentRepo := cockroach.NewComment(DBConn)
	channelRepo := cockroach.NewChannel(DBConn)

	// запускаем сервисы usecase (бизнес-логика)
	telegramUseCase, err := service.NewTelegram(telegramBotToken, postRepo, userRepo, uploadRepo, commentRepo, channelRepo)
	if err != nil {
		log.Fatalf("Ошибка при создании сервиса Telegram (возможно, бот занят или предоставлен невалидный токен): %v", err)
	}
	postUseCase := service.NewPost(postRepo, userRepo, telegramUseCase, nil)
	userUseCase := service.NewUser(userRepo)
	uploadUseCase := service.NewUpload(uploadRepo)

	// запускаем сервисы delivery (обработка запросов)
	cookieManager := utils.NewCookieManager(false)
	postDelivery := delivery.NewPost(cookieManager, postUseCase)
	userDelivery := delivery.NewUser(userUseCase, cookieManager)
	uploadDelivery := delivery.NewUpload(uploadUseCase, userUseCase, cookieManager)
	commentDelivery := delivery.NewComment(telegramUseCase)

	// REST API
	echoServer := echo.New()
	// echoServer.Server.ReadTimeout = time.Duration(coreParams.HTTP.Server.ReadTimeout) * time.Second
	// echoServer.Server.ReadHeaderTimeout = time.Duration(coreParams.HTTP.Server.ReadTimeout) * time.Second
	// echoServer.Server.WriteTimeout = time.Duration(coreParams.HTTP.Server.WriteTimeout) * time.Second
	// echoServer.Server.IdleTimeout = time.Duration(coreParams.HTTP.Server.ReadTimeout) * time.Second

	// Endpoints
	api := echoServer.Group("/api")
	// posts
	posts := api.Group("/posts")
	postDelivery.Configure(posts)
	// users
	users := api.Group("/user")
	userDelivery.Configure(users)
	// uploads
	uploads := api.Group("/upload")
	uploadDelivery.Configure(uploads)
	// comments
	comments := api.Group("/comment")
	commentDelivery.Configure(comments)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()
	go func(server *echo.Echo) {
		if err := server.Start("0.0.0.0:80"); err != nil && !errors.Is(err, http.ErrServerClosed) {
			server.Logger.Fatalf("Сервер завершил свою работу по причине: %v\n", err)
		}
	}(echoServer)

	<-ctx.Done()
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(10)*time.Second,
	)
	defer cancel()
	if err := echoServer.Shutdown(ctx); err != nil {
		echoServer.Logger.Fatalf("Во время выключения сервера возникла ошибка: %s\n", err)
	}
}
