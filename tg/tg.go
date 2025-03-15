package main

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	// Замените на токен вашего бота
	bot, err := tgbotapi.NewBotAPI("token")
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true // Включаем дебаг-режим (по желанию)

	// Настройка long polling
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		// Проверяем, является ли сообщение комментарием (ответом в теме обсуждения)
		if update.Message != nil && update.Message.Chat.IsSuperGroup() {
			fmt.Printf("Новый комментарий в обсуждении: %s\n", update.Message.Text)
		}
	}
}
