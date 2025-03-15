package main

import (
	"github.com/labstack/echo/v4"
	"postic-backend/internal/delivery/http"
	"postic-backend/internal/delivery/platform"
)

func main() {
	// инициализируем tg
	tg, err := platform.NewTg("")
	if err != nil {
		panic(err)
	}
	go tg.Listen()

	// инициализируем vk
	vk, err := platform.NewVk()
	if err != nil {
		panic(err)
	}

	// создаем сервер echo
	echoServer := echo.New()
	// Endpoints
	api := echoServer.Group("/api")
	comments := api.Group("/comments")

	var CommentDelivery = http.NewComment()
	CommentDelivery.Configure(comments, tg, vk)

	echoServer.Logger.Fatal(echoServer.Start(":8080"))
}
