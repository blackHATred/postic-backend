package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	uploadgrpc "postic-backend/internal/delivery/grpc/upload-service"
	grpc_client "postic-backend/internal/delivery/grpc/user-service"
	delivery "postic-backend/internal/delivery/http"
	"postic-backend/internal/delivery/http/utils"
	"postic-backend/internal/repo/cockroach"
	"postic-backend/internal/repo/kafka"
	"postic-backend/internal/usecase/service"
	"postic-backend/internal/usecase/service/telegram"
	"postic-backend/internal/usecase/service/vkontakte"
	"postic-backend/pkg/connector"
	"postic-backend/pkg/goosehelper"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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

	telegramBotToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	jwtSecret := os.Getenv("JWT_SECRET")
	dbConnectDSN := os.Getenv("DB_CONNECT_DSN")
	corsOrigin := os.Getenv("CORS_ORIGIN")
	summarizeURL := os.Getenv("SUMMARIZE_URL")
	replyIdeasURL := os.Getenv("REPLY_IDEAS_URL")
	generatePostURL := os.Getenv("GENERATE_POST_URL")
	fixPostTextURL := os.Getenv("FIX_POST_TEXT_URL")
	vkSuccessURL := os.Getenv("VK_FRONTEND_SUCCESS_REDIRECT_URL")
	vkErrorURL := os.Getenv("VK_FRONTEND_ERROR_REDIRECT_URL")
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	userServiceAddr := os.Getenv("USER_SERVICE_ADDR")
	if userServiceAddr == "" {
		userServiceAddr = "localhost:50051" // Адрес по умолчанию
	}
	uploadServiceAddr := os.Getenv("UPLOAD_SERVICE_ADDR")
	if uploadServiceAddr == "" {
		uploadServiceAddr = "localhost:50052"
	}

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

	// запускаем gRPC upload client и usecase
	uploadClient, err := uploadgrpc.NewClient(uploadServiceAddr)
	if err != nil {
		log.Fatalf("Ошибка при создании gRPC клиента для upload service: %v", err)
	}
	defer uploadClient.Close()

	uploadUseCase := service.NewUpload(uploadClient)

	// запускаем сервисы репозиториев (подключение к базе данных)
	eventRepo, err := kafka.NewCommentEventKafkaRepository(strings.Split(kafkaBrokers, ","))
	if err != nil {
		log.Fatalf("Ошибка при создании Kafka репозитория: %v", err)
	}
	userRepo := cockroach.NewUser(DBConn)
	teamRepo := cockroach.NewTeam(DBConn)
	postRepo := cockroach.NewPost(DBConn)
	commentRepo := cockroach.NewComment(DBConn)
	analyticsRepo := cockroach.NewAnalytics(DBConn)

	// запускаем сервисы usecase (бизнес-логика)
	// -- telegram --
	tgBot, err := tgbotapi.NewBotAPI(telegramBotToken)
	if err != nil {
		log.Fatalf("Ошибка при создании Telegram бота: %v", err)
	}
	telegramPostPlatformUseCase := telegram.NewTelegramPost(tgBot, postRepo, teamRepo, uploadUseCase)
	telegramCommentUseCase := telegram.NewTelegramComment(tgBot, commentRepo, teamRepo, uploadUseCase, eventRepo)
	telegramAnalytics := telegram.NewTelegramAnalytics(teamRepo, postRepo, analyticsRepo)
	// -- vk --
	vkPostPlatformUseCase := vkontakte.NewPost(postRepo, teamRepo, uploadUseCase)
	vkCommentUseCase := vkontakte.NewVkontakteComment(commentRepo, teamRepo, uploadUseCase, eventRepo)
	vkAnalytics := vkontakte.NewVkontakteAnalytics(teamRepo, postRepo, analyticsRepo)
	postUseCase := service.NewPostUnion(
		postRepo,
		teamRepo,
		uploadUseCase,
		analyticsRepo,
		telegramPostPlatformUseCase,
		vkPostPlatformUseCase,
		generatePostURL,
		fixPostTextURL,
	)
	// Используем gRPC клиент для user service вместо прямого создания usecase
	userUseCase, err := grpc_client.NewUserServiceClient(userServiceAddr)
	if err != nil {
		log.Fatalf("Ошибка при создании gRPC клиента для user service: %v", err)
	}
	defer func() {
		if closer, ok := userUseCase.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				log.Errorf("Ошибка при закрытии gRPC соединения: %v", err)
			}
		}
	}()

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
	// Health check endpoints
	echoServer.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status":    "healthy",
			"service":   "gateway",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})
	echoServer.GET("/ready", func(c echo.Context) error {
		// Проверяем готовность критических компонентов
		checks := make(map[string]interface{})

		// Проверка базы данных
		if err := DBConn.Ping(); err != nil {
			checks["database"] = map[string]interface{}{
				"status": "unhealthy",
				"error":  err.Error(),
			}
			return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
				"status": "not ready",
				"checks": checks,
			})
		}
		checks["database"] = map[string]string{"status": "healthy"}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":    "ready",
			"service":   "gateway",
			"checks":    checks,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

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
