package main

import (
	"context"
	"errors"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
	"postic-backend/internal/repo/kafka"
	"postic-backend/internal/usecase/service"
	"postic-backend/internal/usecase/service/telegram"
	"postic-backend/internal/usecase/service/vkontakte"
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
	vkClientID := os.Getenv("VK_CLIENT_ID")
	vkClientSecret := os.Getenv("VK_CLIENT_SECRET")
	vkRedirectURL := os.Getenv("VK_REDIRECT_URL")
	vkSuccessURL := os.Getenv("VK_FRONTEND_SUCCESS_REDIRECT_URL")
	vkErrorURL := os.Getenv("VK_FRONTEND_ERROR_REDIRECT_URL")

	vkAuth := utils.NewVKOAuth(vkClientID, vkClientSecret, vkRedirectURL)

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
	eventRepo, err := kafka.NewCommentEventKafkaRepository([]string{"localhost:9092"})
	if err != nil {
		log.Fatalf("Ошибка при создании Kafka репозитория: %v", err)
	}
	userRepo := cockroach.NewUser(DBConn)
	teamRepo := cockroach.NewTeam(DBConn)
	postRepo := cockroach.NewPost(DBConn)
	uploadRepo, err := cockroach.NewUpload(DBConn, minioClient)
	if err != nil {
		log.Fatalf("Ошибка при создании репозитория Upload: %v", err)
	}
	commentRepo := cockroach.NewComment(DBConn)
	analyticsRepo := cockroach.NewAnalytics(DBConn)
	telegramListenerRepo := cockroach.NewTelegramListener(DBConn)
	vkontakteListenerRepo := cockroach.NewVkontakteListener(DBConn)

	// запускаем сервисы usecase (бизнес-логика)
	// -- telegram --
	tgBot, err := tgbotapi.NewBotAPI(telegramBotToken)
	if err != nil {
		log.Fatalf("Ошибка при создании Telegram бота: %v", err)
	}
	telegramPostPlatformUseCase := telegram.NewTelegramPost(tgBot, postRepo, teamRepo, uploadRepo)
	telegramCommentUseCase := telegram.NewTelegramComment(tgBot, commentRepo, teamRepo, uploadRepo, eventRepo)
	telegramEventListener, err := telegram.NewTelegramEventListener(telegramBotToken, false, telegramListenerRepo, teamRepo, postRepo, uploadRepo, commentRepo, analyticsRepo, eventRepo)
	if err != nil {
		log.Fatalf("Ошибка при создании слушателя событий Post: %v", err)
	}
	telegramAnalytics := telegram.NewTelegramAnalytics(teamRepo, postRepo, analyticsRepo)
	// -- vk --
	vkPostPlatformUseCase := vkontakte.NewPost(postRepo, teamRepo, uploadRepo)
	vkCommentUseCase := vkontakte.NewVkontakteComment(commentRepo, teamRepo, uploadRepo, eventRepo)
	vkEventListener := vkontakte.NewVKEventListener(vkontakteListenerRepo, teamRepo, postRepo, uploadRepo, commentRepo, eventRepo)
	vkAnalytics := vkontakte.NewVkontakteAnalytics(teamRepo, postRepo, analyticsRepo)

	postUseCase := service.NewPostUnion(
		postRepo,
		teamRepo,
		uploadRepo,
		telegramPostPlatformUseCase,
		vkPostPlatformUseCase,
	)
	userUseCase := service.NewUser(userRepo, vkAuth)
	uploadUseCase := service.NewUpload(uploadRepo)
	teamUseCase := service.NewTeam(teamRepo)
	commentUseCase := service.NewComment(
		commentRepo,
		postRepo,
		teamRepo,
		telegramCommentUseCase,
		vkCommentUseCase,
		summarizeURL,
		replyIdeasURL,
		eventRepo,
	)
	analyticsUseCase := service.NewAnalytics(analyticsRepo, teamRepo, postRepo, telegramAnalytics, vkAnalytics)

	// запускаем сервисы delivery (обработка запросов)
	cookieManager := utils.NewCookieManager(false)
	authManager := utils.NewAuthManager([]byte(jwtSecret), userRepo, time.Hour*24*365)
	postDelivery := delivery.NewPost(authManager, postUseCase)
	userDelivery := delivery.NewUser(userUseCase, authManager, cookieManager, vkSuccessURL, vkErrorURL)
	uploadDelivery := delivery.NewUpload(uploadUseCase, authManager)
	teamDelivery := delivery.NewTeam(teamUseCase, authManager)
	commentDelivery := delivery.NewComment(sysCtx, commentUseCase, authManager)
	analyticsDelivery := delivery.NewAnalytics(analyticsUseCase, authManager)

	// REST API
	echoServer := echo.New()

	// Следующими параметрами должен управлять прокси-сервер по типу nginx
	// echoServer.Server.ReadTimeout = 60 * time.Second
	// echoServer.Server.ReadHeaderTimeout = 60 * time.Second
	// echoServer.Server.WriteTimeout = 60 * time.Second
	// echoServer.Server.IdleTimeout = 60 * time.Second
	// Не более 50 МБ
	echoServer.Use(middleware.BodyLimit("50M"))
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
	// analytics
	analytics := api.Group("/analytics")
	analyticsDelivery.Configure(analytics)

	go func(server *echo.Echo) {
		if err := server.Start("0.0.0.0:80"); err != nil && !errors.Is(err, http.ErrServerClosed) {
			server.Logger.Errorf("Сервер завершил свою работу по причине: %v\n", err)
		}
	}(echoServer)
	// Запуск слушателя событий Post. Если приходит сигнал завершения, то слушатель останавливается.
	go telegramEventListener.StartListener()
	defer telegramEventListener.StopListener()
	go vkEventListener.StartListener()
	defer vkEventListener.StopListener()

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
