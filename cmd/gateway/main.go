package main

import (
	"github.com/SevereCloud/vksdk/v3/api"
	"github.com/labstack/echo/v4"
	"postic-backend/internal/delivery/http"
	"postic-backend/internal/delivery/platform"
)

const (
	// VKToken - токен для доступа к API VK
	VKToken = "vk1.a.doDQ6ftfhz6B1C2EfCAqOV7VjnJuPYZGY1c7fthyuLKWVry7jJsxh8Dl5LwIAR85zS7IfkXjIrHUtWjGm1gn5xwF3lSu27rh_ulu-dGKKBu2HVEIuy-tjHKURSLTXnvIagXeHxKUkifD4Prt7rMVMhLNxlEZltuJq2gCnTCkgJ3U49WPlBPfazlqu_fQUpOVJZdYxqyaKqqLdYvJkikP4Q"
	// VKGroupID - ID группы VK
	VKGroupID = 178248213
	// TGToken - токен для доступа к API Telegram
	TGToken = "8103674622:AAHN1Gzw8BtJiK4GyH_SAZHuq49U3fJKbk0"
	// SummarizeURL - URL внешнего сервиса суммаризации
	SummarizeURL = "http://localhost:8081/"
)

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
