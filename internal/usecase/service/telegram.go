package service

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/labstack/gommon/log"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"strconv"
	"strings"
	"time"
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
	go tgUC.EventListener()
	return tgUC, nil
}

func (t *Telegram) botProcessForwardedMessage(update tgbotapi.Update) error {
	channel := update.Message.ForwardFromChat
	if !channel.IsChannel() {
		_, err := t.bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Сообщение переслано не из канала"))
		return err
	}
	channelID := channel.ID
	admins, err := t.bot.GetChatAdministrators(tgbotapi.ChatAdministratorsConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: channelID,
		},
	})
	if err != nil {
		_, err = t.bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Не удалось получить список администраторов канала. Проверьте, что бот добавлен в администраторы канала."))
		return err
	}
	isAdmin := false
	for _, admin := range admins {
		if admin.User.ID == t.bot.Self.ID {
			isAdmin = true
			break
		}
	}
	var discussionID int64
	chat, err := t.bot.GetChat(tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{ChatID: channelID},
	})
	if err != nil {
		return err
	}
	if chat.LinkedChatID != 0 {
		discussionID = chat.LinkedChatID
	}
	var isDiscussionAdmin bool
	if discussionID != 0 {
		chatMember, err := t.bot.GetChatMember(tgbotapi.GetChatMemberConfig{
			ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
				ChatID: discussionID,
				UserID: t.bot.Self.ID,
			},
		})
		if err != nil {
			// ошибка может возвращаться в том случае, если бот - не админ в обсуждениях
			isDiscussionAdmin = false
		} else {
			isDiscussionAdmin = chatMember.IsAdministrator()
		}
	}
	var response string
	if isAdmin {
		response = fmt.Sprintf("✅ Бот является админом в указанном канале \"%s\".\n", channel.Title)
	} else {
		response = fmt.Sprintf("❌ Бот НЕ является админом в указанном канале \"%s\"\n", channel.Title)
	}
	if discussionID != 0 {
		if isDiscussionAdmin {
			response += fmt.Sprintf("✅ Бот является админом в обсуждениях. \nID канала: %d\nID обсуждений: %d", channelID, discussionID)
		} else {
			response += fmt.Sprintf("❌ Бот НЕ является админом в обсуждениях.\nID канала: %d\nID обсуждений: %d", channelID, discussionID)
		}
	} else {
		response += fmt.Sprintf("\nID канала: %d\nОбсуждения не найдены", channelID)
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, response)
	_, err = t.bot.Send(msg)
	return err
}

func (t *Telegram) handleCommands(update tgbotapi.Update) error {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

	switch update.Message.Command() {
	case "start":
		msg.Text = "Привет! Я бот, управляющий телеграм-каналами пользователей Postic. " +
			"Используйте команду /help, чтобы увидеть список доступных команд."
	case "help":
		msg.Text = "Чтобы получить ID канала и ID обсуждений канала, перешлите мне из канала любое сообщение.\n" +
			"Сначала убедитесь, что бот добавлен в администраторы канала и обсуждений (если у вас есть обсуждения, " +
			"привязанные к каналу).\n\nСписок доступных команд:\n" +
			"/start - Начать работу с ботом\n" +
			"/help - Показать список команд\n" +
			"/add_channel - Добавить канал"
	case "add_channel":
		args := update.Message.CommandArguments()
		params := strings.Split(args, " ")
		if len(params) != 3 && len(params) != 2 {
			msg.Text = "Неверное количество параметров. Используйте: /add_channel <ключ пользователя> <ID канала> <ID обсуждений (при наличии)>.\n" +
				"Чтобы узнать, как получить ID канала и ID обсуждений, можете воспользоваться командой /help.\n" +
				"Примеры использования:\n" +
				"`/add_channel token123456 -123456789` - если у вас нет обсуждений\n" +
				"`/add_channel token123456 -123456789 -123456789` - если у вас есть обсуждения"
			_, err := t.bot.Send(msg)
			return err
		}
		secretKey := params[0]
		channelID, err := strconv.ParseInt(params[1], 10, 64)
		if err != nil || channelID >= 0 {
			msg.Text = "Неверный формат channel_id. Используйте целое отрицательное число."
			_, err := t.bot.Send(msg)
			return err
		}
		discussionID, err := strconv.ParseInt(params[2], 10, 64)
		if err != nil || discussionID >= 0 {
			msg.Text = "Неверный формат discussion_id. Используйте целое отрицательное число."
			_, err := t.bot.Send(msg)
			return err
		}
		user, err := t.userRepo.GetUserBySecret(secretKey)
		if err != nil {
			msg.Text = "Неверный секретный ключ."
			_, err := t.bot.Send(msg)
			return err
		}
		err = t.userRepo.PutTGChannel(user.ID, int(channelID), int(discussionID))
		if err != nil {
			msg.Text = "Не удалось добавить канал. Обратитесь в поддержку для решения вопроса."
			_, err := t.bot.Send(msg)
			return err
		}
		msg.Text = "Канал успешно добавлен. Перейдите в личный кабинет и обновите страницу."
	default:
		msg.Text = "Неизвестная команда. Используйте /help, чтобы увидеть список доступных команд."
	}

	_, err := t.bot.Send(msg)
	return err
}

func (t *Telegram) botProcessUpdate(update tgbotapi.Update) error {
	if update.Message != nil && update.Message.ForwardFromChat != nil && update.Message.Chat.IsPrivate() {
		// Пересланное сообщение
		return t.botProcessForwardedMessage(update)
	}
	if update.Message != nil && update.Message.ForwardFrom != nil {
		_, err := t.bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Сообщение переслано не из канала"))
		return err
	}
	if update.Message != nil && update.Message.Command() != "" {
		// Обработка команд
		return t.handleCommands(update)
	}
	return nil
}

func (t *Telegram) EventListener() {
	lastUpdateID, err := t.postRepo.GetLastUpdateTG()
	for err != nil {
		// Пытаемся постоянно получить последний event
		log.Errorf("Telegram GetLastUpdateTG failed: %v", err)
		time.Sleep(1 * time.Second)
		lastUpdateID, err = t.postRepo.GetLastUpdateTG()
	}

	u := tgbotapi.NewUpdate(lastUpdateID + 1)
	u.Timeout = 60
	updates := t.bot.GetUpdatesChan(u)
	for update := range updates {
		if update.Message != nil {
			err = t.botProcessUpdate(update)
			if err != nil {
				log.Errorf("Failed to process update: %v", err)
			}
			err = t.postRepo.SetLastUpdateTG(update.UpdateID)
			if err != nil {
				log.Errorf("Failed to set last update: %v", err)
			}
		}
	}
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
