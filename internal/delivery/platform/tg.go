package platform

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"sync"
	"time"
)

type Tg struct {
	bot         *tgbotapi.BotAPI
	chats       map[int64]chan entity.Message
	mu          sync.Mutex
	commentRepo repo.Comment
}

func NewTg(token string) (*Tg, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	bot.Debug = true
	return &Tg{bot: bot, chats: make(map[int64]chan entity.Message)}, nil
}

func (t *Tg) AddChat(chatID int64) <-chan entity.Message {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.chats[chatID] = make(chan entity.Message)
	fmt.Printf("Chat %d added\n", chatID)
	return t.chats[chatID]
}

func (t *Tg) RemoveChat(chatID int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.chats, chatID)
}

func (t *Tg) ListenMock() error {
	// с помощью ticker время от времени отправляем сообщения в канал
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		for _, ch := range t.chats {
			ch <- entity.Message{Id: 1, Text: "Hello!", Type: "new", Username: "Username", Time: "2021-10-01T12:00:00Z"}
		}
	}
	return nil
}

func (t *Tg) Listen() error {
	// Настройка long polling
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := t.bot.GetUpdatesChan(u)

	for update := range updates {
		// Проверяем, является ли сообщение комментарием (ответом в теме обсуждения)
		fmt.Printf("update: %v\n", update)
		var chatId int64
		var msg *tgbotapi.Message
		switch {
		case update.Message != nil:
			msg = update.Message
		case update.EditedMessage != nil:
			msg = update.EditedMessage
		default:
			continue
		}
		if !msg.Chat.IsSuperGroup() {
			continue
		}
		chatId = msg.Chat.ID
		t.mu.Lock()
		if _, ok := t.chats[chatId]; (update.Message != nil || update.EditedMessage != nil) && ok {
			avatarURL := ""
			/*
				UNSAFE!
					photos, err := t.bot.GetUserProfilePhotos(tgbotapi.UserProfilePhotosConfig{UserID: msg.From.ID, Limit: 1})
					if err != nil {
						fmt.Println(err) // можно проигнорировать
					}
					avatarURL := ""
					if len(photos.Photos) > 0 {
						avatarURL = photos.Photos[0][0].FileID
						file, err := t.bot.GetFile(tgbotapi.FileConfig{FileID: avatarURL})
						if err != nil {
							fmt.Println(err) // можно проигнорировать
						}
						avatarURL = file.Link("")
					}
			*/
			switch {
			// Новое сообщение
			case update.Message != nil:
				t.chats[chatId] <- entity.Message{
					Id:        1,
					Text:      update.Message.Text,
					Type:      "new",
					Username:  update.Message.From.UserName,
					Time:      update.Message.Time().String(),
					Platform:  "tg",
					AvatarURL: avatarURL,
				}
				// Обновление сообщения
			case update.EditedMessage != nil:
				t.chats[chatId] <- entity.Message{
					Id:        1,
					Text:      update.EditedMessage.Text,
					Type:      "update",
					Username:  update.EditedMessage.From.UserName,
					Time:      update.EditedMessage.Time().String(),
					Platform:  "tg",
					AvatarURL: avatarURL,
				}
			}
			fmt.Printf("Новый комментарий в обсуждении %d: %s\n", chatId, msg.Text)
		}
		t.mu.Unlock()
	}
	return nil
}
