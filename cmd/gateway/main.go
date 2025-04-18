package main

import (
	"context"
	"errors"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"net/http"
	"os"
	"os/signal"
	delivery "postic-backend/internal/delivery/http"
	"postic-backend/internal/delivery/http/utils"
	"postic-backend/internal/repo/cockroach"
	"postic-backend/internal/usecase/service"
	"postic-backend/internal/usecase/service/telegram"
	"postic-backend/pkg/connector"
	"strings"
	"time"
)

func main() {
	sysCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()

	err := godotenv.Load()
	if err != nil {
		log.Info(".env файл не обнаружен")
	}
	telegramBotToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	jwtSecret := os.Getenv("JWT_SECRET")
	dbConnectDSN := os.Getenv("DB_CONNECT_DSN")
	minioEndpoint := os.Getenv("MINIO_ENDPOINT")
	minioAccessKey := os.Getenv("MINIO_ACCESS_KEY")
	minioSecretKey := os.Getenv("MINIO_SECRET_KEY")
	minioUseSSL := false
	corsOrigin := os.Getenv("CORS_ORIGIN")
	summarizeURL := os.Getenv("SUMMARIZE_URL")
	replyIdeasURL := os.Getenv("REPLY_IDEAS_URL")

	// cockroach
	DBConn, err := connector.GetCockroachConnector(dbConnectDSN) // примерный вид dsn: "user=root dbname=defaultdb sslmode=disable"
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
	minioClient, err := connector.GetMinioConnector(minioEndpoint, minioAccessKey, minioSecretKey, minioUseSSL)
	if err != nil {
		log.Fatalf("Ошибка при подключении к MinIO: %v", err)
	}

	// запускаем сервисы репозиториев (подключение к базе данных)
	userRepo := cockroach.NewUser(DBConn)
	teamRepo := cockroach.NewTeam(DBConn)
	postRepo := cockroach.NewPost(DBConn)
	uploadRepo, err := cockroach.NewUpload(DBConn, minioClient)
	if err != nil {
		log.Fatalf("Ошибка при создании репозитория Upload: %v", err)
	}
	commentRepo := cockroach.NewComment(DBConn)
	telegramListenerRepo := cockroach.NewTelegramListener(DBConn)

	// запускаем сервисы usecase (бизнес-логика)
	telegramPostPlatformUseCase, err := telegram.NewTelegram(telegramBotToken, postRepo, teamRepo, uploadRepo)
	if err != nil {
		log.Fatalf("Ошибка при создании Telegram UseCase: %v", err)
	}
	telegramCommentUseCase, err := telegram.NewTelegramComment(telegramBotToken, commentRepo, teamRepo, uploadRepo)
	if err != nil {
		log.Fatalf("Ошибка при создании Telegram Comment UseCase: %v", err)
	}
	telegramEventListener, err := telegram.NewEventListener(telegramBotToken, false, telegramListenerRepo, teamRepo, postRepo, uploadRepo, commentRepo)
	if err != nil {
		log.Fatalf("Ошибка при создании слушателя событий Telegram: %v", err)
	}
	postUseCase := service.NewPostUnion(postRepo, teamRepo, uploadRepo, telegramPostPlatformUseCase)
	userUseCase := service.NewUser(userRepo)
	uploadUseCase := service.NewUpload(uploadRepo)
	teamUseCase := service.NewTeam(teamRepo)
	commentUseCase := service.NewComment(
		commentRepo,
		postRepo,
		teamRepo,
		telegramEventListener,
		telegramCommentUseCase,
		summarizeURL,
		replyIdeasURL,
	)

	// запускаем сервисы delivery (обработка запросов)
	cookieManager := utils.NewCookieManager(false)
	authManager := utils.NewAuthManager([]byte(jwtSecret), userRepo, time.Hour*24*365)
	postDelivery := delivery.NewPost(authManager, postUseCase)
	userDelivery := delivery.NewUser(userUseCase, authManager, cookieManager)
	uploadDelivery := delivery.NewUpload(uploadUseCase, authManager)
	teamDelivery := delivery.NewTeam(teamUseCase, authManager)
	commentDelivery := delivery.NewComment(sysCtx, commentUseCase, authManager)

	// REST API
	echoServer := echo.New()

	// Следующими параметрами должен управлять прокси-сервер по типу nginx
	// echoServer.Server.ReadTimeout = 60 * time.Second
	// echoServer.Server.ReadHeaderTimeout = 60 * time.Second
	// echoServer.Server.WriteTimeout = 60 * time.Second
	// echoServer.Server.IdleTimeout = 60 * time.Second
	// Не более 20 МБ
	// echoServer.Use(middleware.BodyLimit("20M"))
	// gzip на прием
	// echoServer.Use(middleware.Decompress())
	// gzip на отдачу
	// echoServer.Use(middleware.Gzip())

	// request id
	echoServer.Use(middleware.RequestID())

	// CORS
	echoServer.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			ctx.Response().Header().Set(echo.HeaderAccessControlAllowOrigin, corsOrigin)
			ctx.Response().Header().Set(echo.HeaderAccessControlAllowMethods, strings.Join([]string{
				http.MethodGet,
				http.MethodPut,
				http.MethodPost,
				http.MethodDelete,
				http.MethodOptions,
			}, ","))
			ctx.Response().Header().Set(echo.HeaderAccessControlAllowHeaders, strings.Join([]string{
				echo.HeaderOrigin,
				echo.HeaderAccept,
				echo.HeaderXRequestedWith,
				echo.HeaderContentType,
				echo.HeaderAccessControlRequestMethod,
				echo.HeaderAccessControlRequestHeaders,
				echo.HeaderCookie,
				"X-Csrf",
			}, ","))
			ctx.Response().Header().Set(echo.HeaderAccessControlAllowCredentials, "true")
			ctx.Response().Header().Set(echo.HeaderAccessControlMaxAge, "86400")
			return next(ctx)
		}
	})

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
	// teams
	teams := api.Group("/teams")
	teamDelivery.Configure(teams)

	go func(server *echo.Echo) {
		if err := server.Start("0.0.0.0:80"); err != nil && !errors.Is(err, http.ErrServerClosed) {
			server.Logger.Errorf("Сервер завершил свою работу по причине: %v\n", err)
		}
	}(echoServer)
	// Запуск слушателя событий Telegram. Если приходит сигнал завершения, то слушатель останавливается.
	go telegramEventListener.StartListener()
	defer telegramEventListener.StopListener()

	<-sysCtx.Done()
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(60)*time.Second,
	)
	defer cancel()
	if err := echoServer.Shutdown(ctx); err != nil {
		echoServer.Logger.Errorf("Во время выключения сервера возникла ошибка: %s\n", err)
	}
}
