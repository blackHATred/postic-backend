package service

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/labstack/gommon/log"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
)

type Telegram struct {
	bot         *tgbotapi.BotAPI
	postRepo    repo.Post
	userRepo    repo.User
	uploadRepo  repo.Upload
	postActions chan entity.PostAction
}

func (t *Telegram) AddPostInQueue(postAction entity.PostAction) error {
	t.postActions <- postAction
	return nil
}

func (t *Telegram) postActionQueue() {
	for action := range t.postActions {
		go t.post(action)
	}
}

func (t *Telegram) getMediaGroup(attachments []int, caption string) ([]any, error) {
	var mediaGroup []any
	for i, attachmentID := range attachments {
		upload, err := t.uploadRepo.GetUpload(attachmentID)
		if err != nil {
			log.Errorf("TG POST: GetUpload failed: %v", err)
			return nil, err
		}
		switch upload.FileType {
		case "photo":
			mediaPhoto := tgbotapi.NewInputMediaPhoto(tgbotapi.FileBytes{
				Name:  upload.FilePath,
				Bytes: upload.RawBytes,
			})
			if i == 0 {
				mediaPhoto.Caption = caption
			}
			mediaGroup = append(mediaGroup, mediaPhoto)
		case "video":
			mediaVideo := tgbotapi.NewInputMediaVideo(tgbotapi.FileBytes{
				Name:  upload.FilePath,
				Bytes: upload.RawBytes,
			})
			if i == 0 {
				mediaVideo.Caption = caption
			}
			mediaGroup = append(mediaGroup, mediaVideo)
		case "raw":
			mediaDocument := tgbotapi.NewInputMediaDocument(tgbotapi.FileBytes{
				Name:  upload.FilePath,
				Bytes: upload.RawBytes,
			})
			if i == 0 {
				mediaDocument.Caption = caption
			}
			mediaGroup = append(mediaGroup, mediaDocument)
		default:
			continue
		}
	}
	return mediaGroup, nil
}

func (t *Telegram) post(action entity.PostAction) {
	// Создаём действие на создание поста
	postActionID, err := t.postRepo.AddPostAction(&action)
	if err != nil {
		log.Errorf("TG POST: AddPostAction failed: %v", err)
		return
	}
	postUnion, err := t.postRepo.GetPostUnion(action.PostUnionID)
	if err != nil {
		log.Errorf("TG POST: GetPostUnion failed: %v", err)
		return
	}
	tgChannel, err := t.userRepo.GetTGChannel(postUnion.UserID)
	if err != nil {
		log.Errorf("TG POST: GetTGChannel failed: %v", err)
		return
	}
	var newPost tgbotapi.Message
	log.Printf("TG POST: PostUnion: %v\n", postUnion)
	// Публикуем пост
	switch {
	// Один медиафайл
	case len(postUnion.Attachments) == 1:
		upload, err := t.uploadRepo.GetUpload(postUnion.Attachments[0])
		if err != nil {
			log.Errorf("TG POST: GetUpload failed: %v", err)
			return
		}
		var attachment tgbotapi.Chattable
		switch upload.FileType {
		case "photo":
			photoConfig := tgbotapi.NewPhoto(int64(tgChannel.ChannelID), tgbotapi.FileBytes{
				Name:  upload.FilePath,
				Bytes: upload.RawBytes,
			})
			photoConfig.Caption = postUnion.Text
			attachment = photoConfig
		case "video":
			videoConfig := tgbotapi.NewVideo(int64(tgChannel.ChannelID), tgbotapi.FileBytes{
				Name:  upload.FilePath,
				Bytes: upload.RawBytes,
			})
			videoConfig.Caption = postUnion.Text
			attachment = videoConfig
		case "raw":
			documentConfig := tgbotapi.NewDocument(int64(tgChannel.ChannelID), tgbotapi.FileBytes{
				Name:  upload.FilePath,
				Bytes: upload.RawBytes,
			})
			documentConfig.Caption = postUnion.Text
			attachment = documentConfig
		}
		newPost, err = t.bot.Send(attachment)
		if err != nil {
			log.Errorf("TG POST: Send failed: %v", err)
			// Меняем статус действия на ошибку
			err = t.postRepo.EditPostActionStatus(postActionID, "error", err.Error())
			if err != nil {
				log.Errorf("TG POST: EditPostActionStatus failed: %v", err)
			}
			return
		}
	// От 2 до 10 картинок или видео
	case len(postUnion.Attachments) > 1 && len(postUnion.Attachments) <= 10:
		mediaGroup, err := t.getMediaGroup(postUnion.Attachments, postUnion.Text)
		if err != nil {
			log.Errorf("TG POST: getMediaGroup failed: %v", err)
			return
		}
		mediaGroupConfig := tgbotapi.NewMediaGroup(int64(tgChannel.ChannelID), mediaGroup)
		newPost, err = t.bot.Send(mediaGroupConfig)
		if err != nil {
			log.Errorf("TG POST: Send failed: %v", err)
			// Меняем статус действия на ошибку
			err = t.postRepo.EditPostActionStatus(postActionID, "error", err.Error())
			if err != nil {
				log.Errorf("TG POST: EditPostActionStatus failed: %v", err)
			}
			return
		}
	// Текстовое сообщение без медиа
	case len(postUnion.Attachments) == 0 && postUnion.Text != "":
		postMessage := tgbotapi.NewMessage(int64(tgChannel.ChannelID), postUnion.Text)
		newPost, err = t.bot.Send(postMessage)
		if err != nil {
			log.Errorf("TG POST: Send failed: %v", err)
			// Меняем статус действия на ошибку
			err = t.postRepo.EditPostActionStatus(postActionID, "error", err.Error())
			if err != nil {
				log.Errorf("TG POST: EditPostActionStatus failed: %v", err)
			}
			return
		}
	// Остальные случаи - не поддерживаются
	default:
		log.Errorf("TG POST: Unsupported post type")
		err = t.postRepo.EditPostActionStatus(postActionID, "error", "Для публикации необходимо от 1 до 10 медиафайлов или текстовое содержание")
		if err != nil {
			log.Errorf("TG POST: EditPostActionStatus failed: %v", err)
		}
		return
	}
	// Изменяем статус действия на успешный
	err = t.postRepo.EditPostActionStatus(postActionID, "success", "")
	if err != nil {
		log.Errorf("TG POST: EditPostActionStatus failed: %v", err)
	}
	// Сохраняем ID поста
	err = t.postRepo.AddPostTG(postUnion.ID, newPost.MessageID)
	if err != nil {
		log.Errorf("TG POST: AddPostTG failed: %v", err)
	}
}

func NewTelegram(token string, postRepo repo.Post, userRepo repo.User, uploadRepo repo.Upload) (usecase.Platform, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	bot.Debug = true
	log.Infof("Authorized on account %s", bot.Self.UserName)
	tgUC := &Telegram{
		bot:         bot,
		postRepo:    postRepo,
		userRepo:    userRepo,
		uploadRepo:  uploadRepo,
		postActions: make(chan entity.PostAction),
	}
	go tgUC.postActionQueue()
	return tgUC, nil
}

/*
func (t *Telegram) Listen() error {
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
*/
