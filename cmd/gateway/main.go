package main

import (
	"github.com/SevereCloud/vksdk/v3/api"
	"github.com/labstack/echo/v4"
	"postic-backend/internal/delivery/http"
	"postic-backend/internal/delivery/platform"
)

const ()

func main() {
	// инициализируем tg
	tg, err := platform.NewTg(TGToken)
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
	apiServer := echoServer.Group("/api")
	comments := apiServer.Group("/comments")

	// инициализируем сервис комментариев
	// vk
	vkApi := api.NewVK(VKToken)
	var CommentDelivery = http.NewComment(nil, tg, vk, SummarizeURL, vkApi)
	CommentDelivery.Configure(comments)

	echoServer.Logger.Fatal(echoServer.Start(":8080"))
}
