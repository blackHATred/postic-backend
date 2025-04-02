package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"github.com/labstack/gommon/log"
	"google.golang.org/grpc"
	"net"
	"os"
	"os/signal"
	"postic-backend/internal/delivery/grpc/comments"
	commentsProto "postic-backend/internal/delivery/grpc/comments/proto"
	"postic-backend/internal/repo/cockroach"
	"postic-backend/internal/usecase/service/telegram"
	"postic-backend/pkg/connector"
	"syscall"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Info(".env файл не обнаружен")
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")

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

	postRepo := cockroach.NewPost(DBConn)
	uploadRepo, err := cockroach.NewUpload(DBConn, minioClient)
	if err != nil {
		log.Fatalf("Ошибка при создании репозитория Upload: %v", err)
	}
	commentRepo := cockroach.NewComment(DBConn)
	telegramListenerRepo := cockroach.NewTelegramListener(DBConn)
	teamRepo := cockroach.NewTeam(DBConn)

	eventListener, err := telegram.NewEventListener(botToken, true, telegramListenerRepo, teamRepo, postRepo, uploadRepo, commentRepo)
	if err != nil {
		log.Fatalf("Ошибка при создании слушателя событий Telegram: %v", err)
	}

	// Запуск GRPC
	grpcServer := grpc.NewServer()
	commentsProto.RegisterCommentsServer(grpcServer, comments.NewGrpc(eventListener))
	addr := fmt.Sprintf("%s:%d", "localhost", 8000)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Невозможно прослушать порт: %v", err)
	}
	log.Infof("Слушаем grpc по адресу %s", addr)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Ошибка при запуске gRPC сервера: %v", err)
		}
	}()

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGKILL)

	// Запуск слушателя событий Telegram. Если приходит сигнал завершения, то слушатель останавливается.
	go eventListener.StartListener()
	for {
		select {
		case <-sigchan:
			log.Info("Получен сигнал завершения.")
			log.Info("Остановка слушателя событий Telegram.")
			eventListener.StopListener()
			log.Info("Остановка gRPC сервера.")
			grpcServer.GracefulStop()
			return
		}
	}
}
