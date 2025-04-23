package vkontakte

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"postic-backend/internal/repo"
)

type Post struct {
	bot        *tgbotapi.BotAPI
	postRepo   repo.Post
	teamRepo   repo.Team
	uploadRepo repo.Upload
}
