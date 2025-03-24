package main

import (
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
	DBConn, err := connector.GetPostgresConnector("") // примерный вид dsn: "user=root dbname=defaultdb sslmode=disable"
	if err != nil {
		log.Fatalf("Ошибка при подключении к базе данных: %v", err)
	}
}
