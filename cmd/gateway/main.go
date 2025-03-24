package main

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"log"
	"postic-backend/pkg/connector"
)

func main() {
	// запускаем сервис для взаимодействия с телеграм (бот для events + actions)
	// запускаем сервис для взаимодействия с вконтакте (множество long polling + actions)
	// запускаем сервисы репозиториев (подключение к базе данных)
	// запускаем сервисы usecase (бизнес-логика)
	// запускаем сервисы delivery (обработка запросов)
	// DBConn
	DBConn, err := connector.GetPostgresConnector("user=root dbname=defaultdb sslmode=disable") // примерный вид dsn: "user=root dbname=defaultdb sslmode=disable"
	if err != nil {
		log.Fatalf("Ошибка при подключении к базе данных: %v", err)
	}
	defer func(DBConn *sqlx.DB) {
		err := DBConn.Close()
		if err != nil {
			log.Fatalf("Ошибка при закрытии соединения с базой данных: %v", err)
		}
	}(DBConn)
}
