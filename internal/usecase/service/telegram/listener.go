package telegram

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/google/uuid"
	"github.com/labstack/gommon/log"
	"io"
	"net/http"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"strconv"
	"strings"
	"sync"
	"time"
)

type EventListener struct {
	bot                       *bot.Bot
	ctx                       context.Context
	cancel                    context.CancelFunc
	telegramEventListenerRepo repo.TelegramListener
	teamRepo                  repo.Team
	postRepo                  repo.Post
	uploadRepo                repo.Upload
	commentRepo               repo.Comment
	analyticsRepo             repo.Analytics
	eventRepo                 repo.CommentEventRepository
	mu                        sync.Mutex
}

func NewTelegramEventListener(
	token string,
	debug bool,
	telegramEventListenerRepo repo.TelegramListener,
	teamRepo repo.Team,
	postRepo repo.Post,
	uploadRepo repo.Upload,
	commentRepo repo.Comment,
	analyticsRepo repo.Analytics,
	eventRepo repo.CommentEventRepository,
) (usecase.Listener, error) {
	lastUpdateID, err := telegramEventListenerRepo.GetLastUpdate()
	for err != nil {
		// Пытаемся постоянно получить последний event
		log.Errorf("Post GetLastUpdate failed: %v", err)
		time.Sleep(1 * time.Second)
		lastUpdateID, err = telegramEventListenerRepo.GetLastUpdate()
	}
	opts := []bot.Option{
		bot.WithInitialOffset(int64(lastUpdateID)),
		bot.WithAllowedUpdates([]string{
			"message",                // Обычные сообщения
			"edited_message",         // Отредактированные сообщения
			"message_reaction",       // Реакции на сообщения
			"message_reaction_count", // Количество реакций
		}),
	}
	if debug {
		opts = append(opts, bot.WithDebug())
	}

	telegramBot, err := bot.New(token, opts...)
	if err != nil {
		return nil, err
	}

	// Создаем контекст с возможностью отмены
	ctx, cancel := context.WithCancel(context.Background())

	// Получаем информацию о боте
	botInfo, err := telegramBot.GetMe(ctx)
	if err != nil {
		cancel()
		return nil, err
	}

	log.Infof("Authorized on account %s", botInfo.Username)

	return &EventListener{
		bot:                       telegramBot,
		ctx:                       ctx,
		cancel:                    cancel,
		telegramEventListenerRepo: telegramEventListenerRepo,
		teamRepo:                  teamRepo,
		postRepo:                  postRepo,
		uploadRepo:                uploadRepo,
		commentRepo:               commentRepo,
		analyticsRepo:             analyticsRepo,
		eventRepo:                 eventRepo,
	}, nil
}

func (t *EventListener) StartListener() {
	// Настраиваем параметры для получения обновлений
	t.bot.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return update.Message != nil || update.EditedMessage != nil || update.MessageReactionCount != nil
		},
		func(ctx context.Context, bot *bot.Bot, update *models.Update) {
			if update.MessageReactionCount != nil {
				// Обработка реакции на сообщение
				log.Infof("Received reactions: %v", update.MessageReactionCount.Reactions)
				t.UpdateStats(update)
			} else if update.Message != nil || update.EditedMessage != nil {
				err := t.botProcessUpdate(update)
				if err != nil {
					log.Errorf("Failed to process update: %v", err)
				}
			}
		},
	)
	t.bot.Start(context.TODO())
}

func (t *EventListener) StopListener() {
	// Отменяем контекст, что останавливает получение обновлений
	t.cancel()
}

func (t *EventListener) UpdateStats(update *models.Update) {
	tgChannel, err := t.teamRepo.GetTGChannelByDiscussionId(int(update.MessageReactionCount.Chat.ID))
	if errors.Is(err, repo.ErrTGChannelNotFound) {
		log.Debugf("Channel not found for discussion ID: %d", update.MessageReactionCount.Chat.ID)
		return
	}
	post, err := t.postRepo.GetPostPlatformByPost(
		update.MessageReactionCount.MessageID,
		tgChannel.ID,
		"tg",
	)
	switch {
	case errors.Is(err, repo.ErrPostPlatformNotFound):
		// игнорируем такую ошибку
		return
	case err != nil:
		log.Errorf("Failed to get post: %v", err)
		return
	}

	// Подсчитываем общее количество реакций
	totalReactions := 0
	if update.MessageReactionCount.Reactions != nil {
		for _, reaction := range update.MessageReactionCount.Reactions {
			totalReactions += reaction.TotalCount
		}
	}

	// Обновляем количество реакций под статистикой
	stats := &entity.PostPlatformStats{
		TeamID:      tgChannel.TeamID,
		PostUnionID: post.PostUnionId,
		Platform:    "tg",
		Reactions:   totalReactions,
	}

	log.Infof("Reactions: %v", stats.Reactions)
	err = t.analyticsRepo.UpdateLastPlatformStats(stats, "tg")
	if err != nil {
		log.Errorf("failed to update post platform stats: %v", err)
	}
}

func getExtensionForType(fileType string) string {
	switch fileType {
	case "photo":
		return "jpg"
	case "video":
		return "mp4"
	case "audio":
		return "mp3"
	case "voice":
		return "ogg"
	case "document":
		return "bin"
	case "sticker":
		return "webp"
	default:
		return "bin"
	}
}

func (t *EventListener) saveFile(fileID, fileType string) (int, error) {
	file, err := t.bot.GetFile(t.ctx, &bot.GetFileParams{
		FileID: fileID,
	})
	if err != nil {
		log.Errorf("Failed to get file: %v", err)
		return 0, err
	}

	// Получаем содержимое файла
	url := t.bot.FileDownloadLink(file)
	resp, err := http.Get(url)
	if err != nil {
		log.Errorf("Failed to get file content: %v", err)
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	// Проверка на TGS-стикер
	isTGS := fileType == "sticker" && strings.HasSuffix(file.FilePath, ".tgs")

	var extension string
	var body io.Reader
	body = resp.Body

	if file.FilePath != "" && strings.Contains(file.FilePath, ".") {
		parts := strings.Split(file.FilePath, ".")
		extension = parts[len(parts)-1]
	} else {
		extension = getExtensionForType(fileType)
	}

	if isTGS {
		// Читаем данные стикера
		tgsData, err := io.ReadAll(body)
		if err != nil {
			log.Errorf("Failed to read sticker data: %v", err)
			return 0, err
		}
		gzipReader, err := gzip.NewReader(bytes.NewReader(tgsData))
		if err != nil {
			log.Errorf("Failed to create gzip reader: %v", err)
			return 0, err
		}
		defer func() { _ = gzipReader.Close() }()
		lottieJSON, err := io.ReadAll(gzipReader)
		if err != nil {
			log.Errorf("Failed to read lottie JSON data: %v", err)
			return 0, err
		}
		body = bytes.NewReader(lottieJSON)
		extension = "json"
	}

	bodyData, err := io.ReadAll(body)
	if err != nil {
		log.Errorf("Failed to read file data: %v", err)
		return 0, err
	}

	// Сохраняем в S3
	upload := &entity.Upload{
		RawBytes: bytes.NewReader(bodyData),
		FilePath: fmt.Sprintf("tg/%s.%s", uuid.New().String(), extension),
		FileType: fileType,
	}
	uploadFileId, err := t.uploadRepo.UploadFile(upload)
	if err != nil {
		log.Errorf("Failed to upload file: %v", err)
		return 0, err
	}
	return uploadFileId, nil
}

func (t *EventListener) handleForwardedMessage(update *models.Update) error {
	channel := update.Message.ForwardOrigin
	if channel.Type != models.MessageOriginTypeChannel {
		_, err := t.bot.SendMessage(t.ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Сообщение переслано не из канала",
		})
		return err
	}
	channelID := channel.MessageOriginChannel.Chat.ID
	admins, err := t.bot.GetChatAdministrators(t.ctx, &bot.GetChatAdministratorsParams{
		ChatID: channelID,
	})
	if err != nil {
		_, err = t.bot.SendMessage(t.ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Не удалось получить список администраторов канала. Проверьте, что бот добавлен в администраторы канала.",
		})
		return err
	}
	// Получаем информацию о боте
	botInfo, err := t.bot.GetMe(t.ctx)
	if err != nil {
		return err
	}
	isAdmin := false
	for _, admin := range admins {
		if admin.Administrator.User.ID == botInfo.ID {
			isAdmin = true
			break
		}
	}
	// Получаем информацию о чате
	chat, err := t.bot.GetChat(t.ctx, &bot.GetChatParams{
		ChatID: channelID,
	})
	if err != nil {
		return err
	}
	var discussionID int64
	if chat.LinkedChatID != 0 {
		discussionID = chat.LinkedChatID
	}
	if chat.LinkedChatID != 0 {
		discussionID = chat.LinkedChatID
	}
	var isDiscussionAdmin bool
	if discussionID != 0 {
		chatMember, err := t.bot.GetChatMember(t.ctx, &bot.GetChatMemberParams{
			ChatID: discussionID,
			UserID: botInfo.ID,
		})
		if err != nil {
			isDiscussionAdmin = false
		} else {
			isDiscussionAdmin = chatMember.Type == models.ChatMemberTypeAdministrator ||
				chatMember.Type == models.ChatMemberTypeOwner
		}
	}

	// Формирование ответа
	var response string
	if isAdmin {
		response = fmt.Sprintf("✅ Бот является админом в указанном канале \"%s\".\n", channel.MessageOriginChannel.Chat.Title)
	} else {
		response = fmt.Sprintf("❌ Бот НЕ является админом в указанном канале \"%s\"\n", channel.MessageOriginChannel.Chat.Title)
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

	_, err = t.bot.SendMessage(t.ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   response,
	})
	return err
}

func (t *EventListener) handleCommand(update *models.Update) error {
	command := strings.Split(update.Message.Text, " ")[0][1:] // Получаем команду без '/'
	args := strings.TrimPrefix(update.Message.Text, "/"+command+" ")

	// Создаем параметры для ответа
	params := &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
	}

	switch command {
	case "start":
		params.Text = "❇️ Привет! Я бот, управляющий телеграм-каналами пользователей Postic. " +
			"Используйте команду /help, чтобы увидеть список доступных команд."
	case "help":
		params.Text = "❇️ Чтобы получить ID канала и ID обсуждений канала, перешлите мне из канала любое сообщение.\n" +
			"Сначала убедитесь, что бот добавлен в администраторы канала и обсуждений (если у вас есть обсуждения, " +
			"привязанные к каналу).\n\nСписок доступных команд:\n" +
			"/start - Начать работу с ботом\n" +
			"/help - Показать список команд\n" +
			"/add_channel - Добавить канал. Если канал уже привязан, то вызов этой команды обновит его настройки"
	case "add_channel":
		cmdArgs := strings.Fields(args)
		if len(cmdArgs) > 3 || len(cmdArgs) < 2 {
			params.Text = "❌ Неверное количество параметров. Используйте: " +
				"/add_channel <ключ пользователя> <ID канала> <ID обсуждений (при наличии)>.\n" +
				"Чтобы узнать, как получить ID канала и ID обсуждений, можете воспользоваться командой /help.\n" +
				"Примеры использования:\n" +
				"`/add_channel token123456 -123456789` - если у вас нет обсуждений\n" +
				"`/add_channel token123456 -123456789 -123456789` - если у вас есть обсуждения"
			_, err := t.bot.SendMessage(t.ctx, params)
			return err
		}
		secretKey := cmdArgs[0]
		channelID, err := strconv.ParseInt(cmdArgs[1], 10, 64)
		if err != nil || channelID >= 0 {
			params.Text = "Неверный формат channel_id. Используйте целое отрицательное число."
			_, err := t.bot.SendMessage(t.ctx, params)
			return err
		}

		var discussionIDParsed int64
		if len(cmdArgs) > 2 {
			discussionIDParsed, err = strconv.ParseInt(cmdArgs[2], 10, 64)
			if err != nil || discussionIDParsed >= 0 {
				params.Text = "Неверный формат discussion_id. Используйте целое отрицательное число."
				_, err := t.bot.SendMessage(t.ctx, params)
				return err
			}
		}

		teamId, err := t.teamRepo.GetTeamIDBySecret(secretKey)
		if err != nil {
			params.Text = "Неверный секретный ключ."
			_, err := t.bot.SendMessage(t.ctx, params)
			return err
		}
		discussionIDint := int(discussionIDParsed)
		err = t.teamRepo.PutTGChannel(&entity.TGChannel{
			TeamID:       teamId,
			ChannelID:    int(channelID),
			DiscussionID: &discussionIDint,
		})
		if err != nil {
			params.Text = "Не удалось добавить канал. Обратитесь в поддержку для решения вопроса."
			_, err := t.bot.SendMessage(t.ctx, params)
			return err
		}
		params.Text = "Канал успешно добавлен. Перейдите в личный кабинет и обновите страницу."
	default:
		params.Text = "Неизвестная команда. Используйте /help, чтобы увидеть список доступных команд."
	}

	_, err := t.bot.SendMessage(t.ctx, params)
	return err
}

func (t *EventListener) handleComment(update *models.Update) error {
	// Проверяем, есть ли у нас такой канал
	discussionID := 0
	if update.Message != nil {
		// сообщения от самого тг не учитываем
		if update.Message.From.Username == "" {
			return nil
		}
		discussionID = int(update.Message.Chat.ID)
	} else if update.EditedMessage != nil {
		discussionID = int(update.EditedMessage.Chat.ID)
	} else {
		return nil
	}
	tgChannel, err := t.teamRepo.GetTGChannelByDiscussionId(discussionID)
	if errors.Is(err, repo.ErrTGChannelNotFound) {
		return nil
	}
	if err != nil {
		log.Errorf("Failed to get team ID by discussion ID: %v", err)
		return err
	}

	var post *entity.PostPlatform
	var replyToComment *entity.Comment
	post = nil
	replyToComment = nil
	if update.Message != nil && update.Message.ReplyToMessage != nil {
		// Первый случай: Ответ на пересланное сообщение из канала
		if update.Message.ReplyToMessage.ForwardOrigin != nil &&
			update.Message.ReplyToMessage.ForwardOrigin.MessageOriginChannel != nil {
			post, err = t.postRepo.GetPostPlatformByPost(
				update.Message.ReplyToMessage.ForwardOrigin.MessageOriginChannel.MessageID,
				tgChannel.ID,
				"tg",
			)
			if errors.Is(err, repo.ErrPostPlatformNotFound) {
				// Если это не пост, то возможно это ответ на комментарий
				replyToComment, err = t.commentRepo.GetCommentByPlatformID(update.Message.ReplyToMessage.ID, "tg")
				if errors.Is(err, repo.ErrCommentNotFound) {
					// Такие случаи игнорим
					log.Debugf("Reply target not found as post or comment, ignoring")
				} else if err != nil {
					log.Errorf("Failed to get comment: %v", err)
					return err
				}
			} else if err != nil {
				log.Errorf("Failed to get post_tg: %v", err)
				return err
			}
		} else {
			// Второй случай: Ответ на сообщение в обсуждении
			log.Debugf("Received direct reply to comment: %s", update.Message.ReplyToMessage.Text)
			replyToComment, err = t.commentRepo.GetCommentByPlatformID(update.Message.ReplyToMessage.ID, "tg")
			if errors.Is(err, repo.ErrCommentNotFound) {
				// игнорим такие комментарии
				log.Debugf("Reply target not found as comment, treating as regular comment")
			} else if err != nil {
				log.Errorf("Failed to get reply target comment: %v", err)
				return err
			}
		}
	}

	// Если это редактирование, проверяем существующий комментарий
	if update.EditedMessage != nil {
		log.Debugf("Received edited message: %s", update.EditedMessage.Text)
		update.Message = update.EditedMessage
		existingComment, err := t.commentRepo.GetCommentByPlatformID(update.Message.ID, "tg")
		if errors.Is(err, repo.ErrCommentNotFound) {
			return nil
		}
		if err != nil {
			log.Errorf("Failed to get comment: %v", err)
			return err
		}
		existingComment.Text = update.Message.Text
		if replyToComment != nil {
			existingComment.ReplyToCommentID = replyToComment.ID
			existingComment.PostUnionID = replyToComment.PostUnionID
		}
		existingComment.Attachments, err = t.processAttachments(update)
		if err != nil {
			log.Errorf("Failed to process attachments: %v", err)
			return err
		}
		// Если так вышло, что у сообщения по каким-то причинам нет текста и аттачей, то игнорируем его
		if existingComment.Text == "" && len(existingComment.Attachments) == 0 {
			return nil
		}
		err = t.commentRepo.EditComment(existingComment)
		if err != nil {
			log.Errorf("Failed to update comment: %v", err)
			return err
		}
		postUnionID := 0
		if existingComment.PostUnionID != nil {
			postUnionID = *existingComment.PostUnionID
		}
		// Уведомляем подписчиков
		event := &entity.CommentEvent{
			EventID:    fmt.Sprintf("tg-%d-%d", tgChannel.TeamID, existingComment.ID),
			TeamID:     tgChannel.TeamID,
			PostID:     postUnionID,
			Type:       entity.CommentEdited,
			CommentID:  existingComment.ID,
			OccurredAt: existingComment.CreatedAt,
		}
		err = t.eventRepo.PublishCommentEvent(t.ctx, event)
		if err != nil {
			log.Errorf("Failed to publish comment event: %v", err)
		}

		return nil
	}

	// Создаём комментарий
	teamID, err := t.teamRepo.GetTeamIDByTGDiscussionID(discussionID)
	if errors.Is(err, repo.ErrTGChannelNotFound) {
		log.Errorf("Failed to get team ID by discussion ID: %v", err)
		return nil
	}

	newComment := &entity.Comment{
		TeamID:            teamID,
		Platform:          "tg",
		UserPlatformID:    int(update.Message.From.ID),
		CommentPlatformID: update.Message.ID,
		FullName:          fmt.Sprintf("%s %s", update.Message.From.FirstName, update.Message.From.LastName),
		Username:          update.Message.From.Username,
		Text:              update.Message.Text,
		CreatedAt:         time.Unix(int64(update.Message.Date), 0),
	}
	if replyToComment != nil {
		newComment.ReplyToCommentID = replyToComment.ID
		newComment.PostUnionID = replyToComment.PostUnionID
		newComment.PostPlatformID = replyToComment.PostPlatformID
	} else if post != nil {
		newComment.PostUnionID = &post.PostUnionId
		newComment.PostPlatformID = &post.PostId
	}

	// Загружаем фотку, сохраняем в S3, сохраняем в БД
	photos, err := t.bot.GetUserProfilePhotos(t.ctx, &bot.GetUserProfilePhotosParams{
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
			// Получаем полную информацию о загруженном файле
			upload, err := t.uploadRepo.GetUploadInfo(uploadFileId)
			if err != nil {
				log.Errorf("Failed to get uploaded avatar file: %v", err)
			} else {
				newComment.AvatarMediaFile = upload
			}
		}
	}

	newComment.Attachments, err = t.processAttachments(update)
	if err != nil {
		log.Errorf("Failed to process attachments: %v", err)
		return err
	}
	// Если так вышло, что у сообщения по каким-то причинам нет текста и аттачей, то игнорируем его
	if newComment.Text == "" && len(newComment.Attachments) == 0 {
		return nil
	}
	// Сохраняем комментарий
	tgCommentId, err := t.commentRepo.AddComment(newComment)
	if err != nil {
		log.Errorf("Failed to save comment: %v", err)
		return err
	}
	newComment.ID = tgCommentId
	// Отправляем комментарий подписчикам
	postUnionIDint := 0
	if newComment.PostUnionID != nil {
		postUnionIDint = *newComment.PostUnionID
	}
	event := &entity.CommentEvent{
		EventID:    fmt.Sprintf("tg-%d-%d", tgChannel.TeamID, newComment.ID),
		TeamID:     tgChannel.TeamID,
		PostID:     postUnionIDint,
		Type:       entity.CommentCreated,
		CommentID:  newComment.ID,
		OccurredAt: newComment.CreatedAt,
	}
	err = t.eventRepo.PublishCommentEvent(t.ctx, event)
	if err != nil {
		log.Errorf("Failed to publish comment event: %v", err)
	}

	return nil
}

func (t *EventListener) processAttachments(update *models.Update) ([]*entity.Upload, error) {
	attachments := make([]*entity.Upload, 0)
	if update.Message.Photo != nil && len(update.Message.Photo) > 0 {
		uploadFileId, err := t.saveFile(update.Message.Photo[len(update.Message.Photo)-1].FileID, "photo")
		if err != nil {
			log.Errorf("Failed to save photo: %v", err)
			return nil, err
		}
		upload, err := t.uploadRepo.GetUploadInfo(uploadFileId)
		if err != nil {
			log.Errorf("Failed to get uploaded photo file: %v", err)
			return nil, err
		}
		attachments = append(attachments, upload)
	}
	if update.Message.Video != nil {
		uploadFileId, err := t.saveFile(update.Message.Video.FileID, "video")
		if err != nil {
			log.Errorf("Failed to save video: %v", err)
			return nil, err
		}
		upload, err := t.uploadRepo.GetUploadInfo(uploadFileId)
		if err != nil {
			log.Errorf("Failed to get uploaded video file: %v", err)
			return nil, err
		}
		attachments = append(attachments, upload)
	}
	// Файл не больше 100 мб
	if update.Message.Document != nil && update.Message.Document.FileSize < 100*1024*1024 {
		uploadFileId, err := t.saveFile(update.Message.Document.FileID, "document")
		if err != nil {
			log.Errorf("Failed to save document: %v", err)
			return nil, err
		}
		upload, err := t.uploadRepo.GetUploadInfo(uploadFileId)
		if err != nil {
			log.Errorf("Failed to get uploaded document file: %v", err)
			return nil, err
		}
		attachments = append(attachments, upload)
	}
	if update.Message.Audio != nil {
		uploadFileId, err := t.saveFile(update.Message.Audio.FileID, "audio")
		if err != nil {
			log.Errorf("Failed to save audio: %v", err)
			return nil, err
		}
		upload, err := t.uploadRepo.GetUploadInfo(uploadFileId)
		if err != nil {
			log.Errorf("Failed to get uploaded audio file: %v", err)
			return nil, err
		}
		attachments = append(attachments, upload)
	}
	if update.Message.Voice != nil {
		uploadFileId, err := t.saveFile(update.Message.Voice.FileID, "voice")
		if err != nil {
			log.Errorf("Failed to save voice: %v", err)
			return nil, err
		}
		upload, err := t.uploadRepo.GetUploadInfo(uploadFileId)
		if err != nil {
			log.Errorf("Failed to get uploaded voice file: %v", err)
			return nil, err
		}
		attachments = append(attachments, upload)
	}
	if update.Message.Sticker != nil {
		uploadFileId, err := t.saveFile(update.Message.Sticker.FileID, "sticker")
		if err != nil {
			log.Errorf("Failed to save sticker: %v", err)
			return nil, err
		}
		upload, err := t.uploadRepo.GetUploadInfo(uploadFileId)
		if err != nil {
			log.Errorf("Failed to get uploaded sticker file: %v", err)
			return nil, err
		}
		attachments = append(attachments, upload)
	}
	return attachments, nil
}

func (t *EventListener) botProcessUpdate(update *models.Update) error {
	if update.Message != nil &&
		update.Message.ForwardOrigin != nil &&
		update.Message.ForwardOrigin.MessageOriginChannel != nil &&
		update.Message.Chat.Type == models.ChatTypePrivate {
		// Пересланное сообщение из канала лично боту
		return t.handleForwardedMessage(update)
	}
	if update.Message != nil &&
		update.Message.ForwardOrigin != nil &&
		update.Message.Chat.Type == models.ChatTypePrivate {
		// Пересланное сообщение лично боту, но это сообщение не из канала
		_, err := t.bot.SendMessage(t.ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text: "❌ Сообщение переслано не из канала.\n" +
				"🔍 Пожалуйста, ознакомьтесь с функциями бота с помощью команды /help",
		})
		return err
	}
	if update.Message != nil && update.Message.Text != "" && strings.HasPrefix(update.Message.Text, "/") {
		return t.handleCommand(update)
	}
	// Сообщение в обсуждениях
	if (update.Message != nil && update.Message.Chat.Type != models.ChatTypePrivate) ||
		(update.EditedMessage != nil && update.EditedMessage.Chat.Type != models.ChatTypePrivate) {
		return t.handleComment(update)
	}
	return nil
}
