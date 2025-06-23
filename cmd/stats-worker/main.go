package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"postic-backend/internal/repo/cockroach"
	"postic-backend/internal/usecase/service"
	"postic-backend/internal/usecase/service/telegram"
	"postic-backend/internal/usecase/service/vkontakte"
	"postic-backend/pkg/connector"
	"postic-backend/pkg/goosehelper"

	"github.com/joho/godotenv"
	"github.com/labstack/gommon/log"
)

func init() {
	// Загружаем переменные окружения
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
	workerID := os.Getenv("STATS_WORKER_ID")
	workerIntervalStr := os.Getenv("STATS_WORKER_INTERVAL")

	if dbConnectDSN == "" {
		log.Fatal("DB_CONNECT_DSN переменная окружения обязательна")
	}

	if workerID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			workerID = fmt.Sprintf("stats-worker-%d", time.Now().Unix())
		} else {
			workerID = fmt.Sprintf("stats-worker-%s-%d", hostname, time.Now().Unix())
		}
	}

	// Парсим интервал обновления (по умолчанию 1 минута)
	workerInterval := 1 * time.Minute
	if workerIntervalStr != "" {
		if parsedInterval, err := time.ParseDuration(workerIntervalStr); err == nil {
			workerInterval = parsedInterval
		} else {
			log.Warnf("Неверный формат STATS_WORKER_INTERVAL: %s, используется 1m", workerIntervalStr)
		}
	}

	log.Infof("Запуск воркера обновления статистики с ID: %s, интервал: %s", workerID, workerInterval)

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
	teamRepo := cockroach.NewTeam(dbConn)
	postRepo := cockroach.NewPost(dbConn)
	analyticsRepo := cockroach.NewAnalytics(dbConn)

	// Инициализация платформенных сервисов аналитики
	telegramAnalytics := telegram.NewTelegramAnalytics(teamRepo, postRepo, analyticsRepo)
	vkAnalytics := vkontakte.NewVkontakteAnalytics(teamRepo, postRepo, analyticsRepo)

	// Инициализация основного сервиса аналитики
	analyticsUseCase := service.NewAnalytics(
		analyticsRepo,
		teamRepo,
		postRepo,
		telegramAnalytics,
		vkAnalytics,
	)

	// Создание и запуск воркера
	statsWorker := service.NewStatsWorker(analyticsUseCase, workerID, workerInterval)

	log.Info("Воркер статистики запущен")
	statsWorker.Start(ctx)
	log.Info("Воркер статистики остановлен")
}
