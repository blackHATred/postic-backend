package telegram

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/labstack/gommon/log"
	"net/http"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"strconv"
	"strings"
	"sync"
	"time"
)

type subscriber struct {
	teamId      int
	postUnionId int
}

type EventListener struct {
	bot                       *tgbotapi.BotAPI
	telegramEventListenerRepo repo.TelegramListener
	teamRepo                  repo.Team
	postRepo                  repo.Post
	uploadRepo                repo.Upload
	commentRepo               repo.Comment
	subscribers               map[subscriber]chan int
	mu                        sync.Mutex
}

func NewEventListener(
	token string,
	debug bool,
	telegramEventListenerRepo repo.TelegramListener,
	teamRepo repo.Team,
	postRepo repo.Post,
	uploadRepo repo.Upload,
	commentRepo repo.Comment,
) (usecase.Listener, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	bot.Debug = debug
	log.Infof("Authorized on account %s", bot.Self.UserName)
	return &EventListener{
		bot:                       bot,
		telegramEventListenerRepo: telegramEventListenerRepo,
		teamRepo:                  teamRepo,
		postRepo:                  postRepo,
		uploadRepo:                uploadRepo,
		commentRepo:               commentRepo,
		subscribers:               make(map[subscriber]chan int),
	}, nil
}

func (t *EventListener) StartListener() {
	lastUpdateID, err := t.telegramEventListenerRepo.GetLastUpdate()
	for err != nil {
		// Пытаемся постоянно получить последний event
		log.Errorf("Telegram GetLastUpdate failed: %v", err)
		time.Sleep(1 * time.Second)
		lastUpdateID, err = t.telegramEventListenerRepo.GetLastUpdate()
	}
	u := tgbotapi.NewUpdate(lastUpdateID + 1)
	u.Timeout = 60
	updates := t.bot.GetUpdatesChan(u)
	for update := range updates {
		if update.Message != nil {
			err = t.botProcessUpdate(&update)
			if err != nil {
				log.Errorf("Failed to process update: %v", err)
			}
			err = t.telegramEventListenerRepo.SetLastUpdate(update.UpdateID)
			if err != nil {
				log.Errorf("Failed to set last update: %v", err)
			}
		}
	}
}

func (t *EventListener) StopListener() {
	// это автоматически закрывает канал updates в StartListener
	t.bot.StopReceivingUpdates()
	// закрываем все каналы подписчиков
	t.mu.Lock()
	for _, ch := range t.subscribers {
		close(ch)
	}
	t.subscribers = make(map[subscriber]chan int)
	t.mu.Unlock()
}

func (t *EventListener) SubscribeToCommentEvents(teamId, postUnionId int) <-chan int {
	subscriber := subscriber{
		teamId:      teamId,
		postUnionId: postUnionId,
	}
	ch := make(chan int)
	t.mu.Lock()
	t.subscribers[subscriber] = ch
	t.mu.Unlock()
	return ch
}

func (t *EventListener) UnsubscribeFromComments(teamId, postUnionId int) {
	subscriber := subscriber{
		teamId:      teamId,
		postUnionId: postUnionId,
	}
	t.mu.Lock()
	if ch, ok := t.subscribers[subscriber]; ok {
		close(ch)
		delete(t.subscribers, subscriber)
	}
	t.mu.Unlock()
}

func (t *EventListener) saveFile(fileID, fileType string) (int, error) {
	file, err := t.bot.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		log.Errorf("Failed to get file: %v", err)
		return 0, err
	}
	// Получаем содержимое файла
	url := file.Link(t.bot.Token)
	resp, err := http.Get(url)
	if err != nil {
		log.Errorf("Failed to get file content: %v", err)
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	// Сохраняем в S3
	upload := &entity.Upload{
		RawBytes: resp.Body,
		FilePath: fmt.Sprintf("tg/user_avatars/%s.jpg", uuid.New().String()),
		FileType: fileType,
	}
	uploadFileId, err := t.uploadRepo.UploadFile(upload)
	if err != nil {
		log.Errorf("Failed to upload file: %v", err)
		return 0, err
	}
	return uploadFileId, nil
}

func (t *EventListener) handleForwardedMessage(update *tgbotapi.Update) error {
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
		_, err = t.bot.Send(
			tgbotapi.NewMessage(
				update.Message.Chat.ID,
				"❌ Не удалось получить список администраторов канала. "+
					"Проверьте, что бот добавлен в администраторы канала.",
			),
		)
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
			response += fmt.Sprintf(
				"✅ Бот является админом в обсуждениях. \nID канала: %d\nID обсуждений: %d",
				channelID,
				discussionID,
			)
		} else {
			response += fmt.Sprintf(
				"❌ Бот НЕ является админом в обсуждениях.\nID канала: %d\nID обсуждений: %d",
				channelID,
				discussionID,
			)
		}
	} else {
		response += fmt.Sprintf("\nID канала: %d\nОбсуждения не найдены", channelID)
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, response)
	_, err = t.bot.Send(msg)
	return err
}

func (t *EventListener) handleCommand(update *tgbotapi.Update) error {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	switch update.Message.Command() {
	case "start":
		msg.Text = "❇️ Привет! Я бот, управляющий телеграм-каналами пользователей Postic. " +
			"Используйте команду /help, чтобы увидеть список доступных команд."
	case "help":
		msg.Text = "❇️ Чтобы получить ID канала и ID обсуждений канала, перешлите мне из канала любое сообщение.\n" +
			"Сначала убедитесь, что бот добавлен в администраторы канала и обсуждений (если у вас есть обсуждения, " +
			"привязанные к каналу).\n\nСписок доступных команд:\n" +
			"/start - Начать работу с ботом\n" +
			"/help - Показать список команд\n" +
			"/add_channel - Добавить канал. Если канал уже привязан, то вызов этой команды обновит его настройки"
	case "add_channel":
		args := update.Message.CommandArguments()
		params := strings.Split(args, " ")
		if len(params) > 3 && len(params) < 2 {
			msg.Text = "❌ Неверное количество параметров. Используйте: " +
				"/add_channel <ключ пользователя> <ID канала> <ID обсуждений (при наличии)>.\n" +
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
		teamId, err := t.teamRepo.GetTeamIDBySecret(secretKey)
		if err != nil {
			msg.Text = "Неверный секретный ключ."
			_, err := t.bot.Send(msg)
			return err
		}
		err = t.teamRepo.PutTGChannel(teamId, int(channelID), int(discussionID))
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

func (t *EventListener) handleComment(update *tgbotapi.Update) error {
	// Проверяем, есть ли у нас такой канал
	log.Infof("Update: %v", update)
	_, err := t.teamRepo.GetTGChannelByDiscussionId(int(update.Message.Chat.ID))
	if err != nil {
		// ничего не делаем, просто игнорируем сообщение, если оно не относится к нашим каналам
		return nil
	}
	postTg, err := t.postRepo.GetPostTGByMessageID(update.Message.ReplyToMessage.ForwardFromMessageID)
	if err != nil {
		log.Errorf("Failed to get post_tg: %v", err)
		return err
	}
	// Создаём комментарий
	newComment := &entity.Comment{
		ID:                0,
		PostUnionID:       postTg.PostUnionId,
		Platform:          "tg",
		PostPlatformID:    postTg.PostId,
		UserPlatformID:    int(update.Message.From.ID),
		CommentPlatformID: update.Message.MessageID,
		FullName:          fmt.Sprintf("%s %s", update.Message.From.FirstName, update.Message.From.LastName),
		Username:          update.Message.From.UserName,
		Text:              update.Message.Text,
		CreatedAt:         update.Message.Time(),
	}
	// Загружаем фотку, сохраняем в S3, сохраняем в БД
	photos, err := t.bot.GetUserProfilePhotos(tgbotapi.UserProfilePhotosConfig{
		UserID: update.Message.From.ID,
		Limit:  1,
	})
	if err != nil {
		log.Errorf("Failed to get user profile photos: %v", err)
		// не делаем return - ошибка не критичная, просто не будет аватарки
	}
	if len(photos.Photos) > 0 {
		uploadFileId, err := t.saveFile(photos.Photos[0][0].FileID, "photo")
		if err != nil {
			log.Errorf("Failed to save user profile photo: %v", err)
			// не делаем return - ошибка не критичная, просто не будет аватарки
		} else {
			newComment.AvatarMediaFileID = &uploadFileId
		}
	}
	log.Infof("New newComment: %v", newComment)

	newComment.Attachments = make([]int, 0)
	// Если есть аттачи, то прикрепляем их к комментарию
	if update.Message.Photo != nil {
		uploadFileId, err := t.saveFile(update.Message.Photo[0].FileID, "photo")
		if err != nil {
			log.Errorf("Failed to save photo: %v", err)
			return err
		}
		newComment.Attachments = append(newComment.Attachments, uploadFileId)
	}
	if update.Message.Video != nil {
		uploadFileId, err := t.saveFile(update.Message.Video.FileID, "video")
		if err != nil {
			log.Errorf("Failed to save video: %v", err)
			return err
		}
		newComment.Attachments = append(newComment.Attachments, uploadFileId)
	}
	// Файл не больше 20 мб
	if update.Message.Document != nil && update.Message.Document.FileSize < 20*1024*1024 {
		uploadFileId, err := t.saveFile(update.Message.Document.FileID, "document")
		if err != nil {
			log.Errorf("Failed to save document: %v", err)
			return err
		}
		newComment.Attachments = append(newComment.Attachments, uploadFileId)
	}
	if update.Message.Audio != nil {
		uploadFileId, err := t.saveFile(update.Message.Audio.FileID, "audio")
		if err != nil {
			log.Errorf("Failed to save audio: %v", err)
			return err
		}
		newComment.Attachments = append(newComment.Attachments, uploadFileId)
	}
	if update.Message.Voice != nil {
		uploadFileId, err := t.saveFile(update.Message.Voice.FileID, "voice")
		if err != nil {
			log.Errorf("Failed to save voice: %v", err)
			return err
		}
		newComment.Attachments = append(newComment.Attachments, uploadFileId)
	}
	if update.Message.Sticker != nil {
		uploadFileId, err := t.saveFile(update.Message.Sticker.FileID, "sticker")
		if err != nil {
			log.Errorf("Failed to save sticker: %v", err)
			return err
		}
		newComment.Attachments = append(newComment.Attachments, uploadFileId)
	}
	// Если так вышло, что у сообщения нет текста и аттачей, то игнорируем его
	if newComment.Text == "" && len(newComment.Attachments) == 0 {
		return nil
	}
	// Сохраняем комментарий
	tgCommentId, err := t.commentRepo.AddComment(newComment)
	if err != nil {
		log.Errorf("Failed to save newComment: %v", err)
		return err
	}
	newComment.ID = tgCommentId
	// Отправляем комментарий подписчикам
	return t.notifySubscribers(newComment)
}

func (t *EventListener) botProcessUpdate(update *tgbotapi.Update) error {
	if update.Message != nil && update.Message.ForwardFromChat != nil && update.Message.Chat.IsPrivate() {
		// Пересланное сообщение из канала лично боту
		return t.handleForwardedMessage(update)
	}
	if update.Message != nil && update.Message.ForwardFrom != nil && update.Message.Chat.IsPrivate() {
		// Пересланное сообщение лично боту, но это сообщение не из канала
		_, err := t.bot.Send(
			tgbotapi.NewMessage(
				update.Message.Chat.ID,
				"❌ Сообщение переслано не из канала.\n"+
					"🔍 Пожалуйста, ознакомьтесь с функциями бота с помощью команды /help",
			),
		)
		return err
	}
	if update.Message != nil && update.Message.Command() != "" {
		// Обработка команд
		return t.handleCommand(update)
	}
	// Reply на пост в канале или редактирование этого reply - это комментарий, который нужно сохранить и отправить подписчикам
	if update.Message != nil && update.Message.ReplyToMessage != nil && !update.Message.Chat.IsPrivate() {
		return t.handleComment(update)
	}
	return nil
}

func (t *EventListener) notifySubscribers(comment *entity.Comment) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Определяем, какой команде принадлежит комментарий
	teamId, err := t.teamRepo.GetTeamIDByPostUnionID(comment.PostUnionID)
	if err != nil {
		log.Errorf("Failed to get teamId by postUnionID: %v", err)
		return err
	}
	// Возможны два варианта подписки: на всю ленту комментариев и на комментарии под конкретным постом
	sub1 := subscriber{
		teamId:      teamId,
		postUnionId: comment.PostUnionID,
	}
	sub2 := subscriber{
		teamId:      teamId,
		postUnionId: 0,
	}
	// Отправляем комментарий подписчикам в новых горутинах для избежания блокировок
	if ch, ok := t.subscribers[sub1]; ok {
		go func() {
			ch <- comment.ID
		}()
	}
	if ch, ok := t.subscribers[sub2]; ok {
		go func() {
			ch <- comment.ID
		}()
	}
	return nil
}
