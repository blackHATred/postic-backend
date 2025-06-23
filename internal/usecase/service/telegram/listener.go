package telegram

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/google/uuid"
	"github.com/labstack/gommon/log"
)

type EventListener struct {
	bot                       *bot.Bot
	ctx                       context.Context
	cancel                    context.CancelFunc
	telegramEventListenerRepo repo.TelegramListener
	teamRepo                  repo.Team
	postRepo                  repo.Post
	uploadUseCase             usecase.Upload
	commentRepo               repo.Comment
	analyticsRepo             repo.Analytics
	eventRepo                 repo.CommentEventRepository

	// Буфер для медиагрупп: media_group_id -> []*models.Update
	mediaGroupBuffer map[string][]*models.Update
	// Таймеры для медиагрупп: media_group_id -> *time.Timer
	mediaGroupTimers map[string]*time.Timer
	// Мьютекс для потокобезопасности
	mediaGroupMutex sync.Mutex
}

func NewTelegramEventListener(
	token string,
	debug bool,
	telegramEventListenerRepo repo.TelegramListener,
	teamRepo repo.Team,
	postRepo repo.Post,
	uploadUseCase usecase.Upload,
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
		uploadUseCase:             uploadUseCase,
		commentRepo:               commentRepo,
		analyticsRepo:             analyticsRepo,
		eventRepo:                 eventRepo,
		mediaGroupBuffer:          make(map[string][]*models.Update),
		mediaGroupTimers:          make(map[string]*time.Timer),
	}, nil
}

func (t *EventListener) StartListener() {
	t.setupHandlers()
	t.bot.Start(context.TODO())
}

func (t *EventListener) StopListener() {
	// Отменяем контекст, что останавливает получение обновлений
	t.cancel()
}

// saveLastUpdateID сохраняет ID последнего обработанного обновления
func (t *EventListener) saveLastUpdateID(updateID int) {
	err := t.telegramEventListenerRepo.SetLastUpdate(updateID)
	if err != nil {
		log.Errorf("Failed to save last update ID %d: %v", updateID, err)
	}
}

func (t *EventListener) UpdateStats(update *models.Update) {
	tgChannel, err := t.teamRepo.GetTGChannelByChannelID(int(update.MessageReactionCount.Chat.ID))
	if errors.Is(err, repo.ErrTGChannelNotFound) {
		log.Infof("Channel not found for discussion ID: %d", update.MessageReactionCount.Chat.ID)
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
		log.Infof("Post not found for message ID: %d", update.MessageReactionCount.MessageID)
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
		RecordedAt:  time.Now(),
		Reactions:   totalReactions,
	}

	log.Infof("Reactions: %v", stats.Reactions)
	err = t.analyticsRepo.SavePostPlatformStats(stats)
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
	uploadFileId, err := t.uploadUseCase.UploadFile(upload)
	if err != nil {
		log.Errorf("Failed to upload file: %v", err)
		return 0, err
	}
	return uploadFileId, nil
}

func (t *EventListener) handleForwardedMessage(update *models.Update) error {
	// Проверяем, что это сообщение из канала
	if update.Message.ForwardOrigin.Type != models.MessageOriginTypeChannel {
		_, err := t.bot.SendMessage(t.ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text: "❌ Сообщение переслано не из канала.\n" +
				"🔍 Пожалуйста, ознакомьтесь с функциями бота с помощью команды /help",
		})
		return err
	}

	channel := update.Message.ForwardOrigin.MessageOriginChannel
	channelID := channel.Chat.ID
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
		response = fmt.Sprintf("✅ Бот является админом в указанном канале \"%s\".\n", channel.Chat.Title)
	} else {
		response = fmt.Sprintf("❌ Бот НЕ является админом в указанном канале \"%s\"\n", channel.Chat.Title)
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

func (t *EventListener) processAttachments(update *models.Update) ([]*entity.Upload, error) {
	attachments := make([]*entity.Upload, 0)
	if len(update.Message.Photo) > 0 {
		uploadFileId, err := t.saveFile(update.Message.Photo[len(update.Message.Photo)-1].FileID, "photo")
		if err != nil {
			log.Errorf("Failed to save photo: %v", err)
			return nil, err
		}
		upload, err := t.uploadUseCase.GetUpload(uploadFileId)
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
		upload, err := t.uploadUseCase.GetUpload(uploadFileId)
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
		upload, err := t.uploadUseCase.GetUpload(uploadFileId)
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
		upload, err := t.uploadUseCase.GetUpload(uploadFileId)
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
		upload, err := t.uploadUseCase.GetUpload(uploadFileId)
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
		upload, err := t.uploadUseCase.GetUpload(uploadFileId)
		if err != nil {
			log.Errorf("Failed to get uploaded sticker file: %v", err)
			return nil, err
		}
		attachments = append(attachments, upload)
	}
	return attachments, nil
}

func (t *EventListener) setupHandlers() {
	t.bot.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return update.MessageReactionCount != nil
		},
		func(ctx context.Context, bot *bot.Bot, update *models.Update) {
			t.handleReactionUpdate(ctx, update)
		},
	)

	t.bot.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return update.Message != nil
		},
		func(ctx context.Context, bot *bot.Bot, update *models.Update) {
			t.handleMessageUpdate(ctx, update, false)
		},
	)

	t.bot.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return update.EditedMessage != nil
		},
		func(ctx context.Context, bot *bot.Bot, update *models.Update) {
			t.handleMessageUpdate(ctx, update, true)
		},
	)
}

func (t *EventListener) handleReactionUpdate(ctx context.Context, update *models.Update) {
	log.Infof("Received reactions: %v", update.MessageReactionCount.Reactions)
	t.UpdateStats(update)
	// Сохраняем ID последнего обработанного обновления
	t.saveLastUpdateID(int(update.ID))
}

func (t *EventListener) handleMessageUpdate(ctx context.Context, update *models.Update, isEdit bool) {
	var message *models.Message
	if isEdit {
		message = update.EditedMessage
		update.Message = message // Унифицируем для дальнейшей обработки
	} else {
		message = update.Message
	}

	// Определяем тип сообщения и обрабатываем соответственно
	if t.isPrivateForwardedMessage(message) {
		err := t.handleForwardedMessage(update)
		if err != nil {
			log.Errorf("Failed to handle forwarded message: %v", err)
		}
		// Сохраняем ID последнего обработанного обновления
		t.saveLastUpdateID(int(update.ID))
		return
	}

	if t.isPrivateCommand(message) {
		err := t.handleCommand(update)
		if err != nil {
			log.Errorf("Failed to handle command: %v", err)
		}
		// Сохраняем ID последнего обработанного обновления
		t.saveLastUpdateID(int(update.ID))
		return
	}

	if t.isGroupMessage(message) {
		if isEdit {
			err := t.handleCommentEdit(ctx, update)
			if err != nil {
				log.Errorf("Failed to handle comment edit: %v", err)
			}
		} else {
			err := t.handleNewComment(ctx, update)
			if err != nil {
				log.Errorf("Failed to handle new comment: %v", err)
			}
		}
		// Сохраняем ID последнего обработанного обновления
		t.saveLastUpdateID(int(update.ID))
		return
	}
}

func (t *EventListener) isPrivateForwardedMessage(message *models.Message) bool {
	return message.ForwardOrigin != nil && message.Chat.Type == models.ChatTypePrivate
}

func (t *EventListener) isPrivateCommand(message *models.Message) bool {
	return message.Text != "" && strings.HasPrefix(message.Text, "/") && message.Chat.Type == models.ChatTypePrivate
}

func (t *EventListener) isGroupMessage(message *models.Message) bool {
	return message.Chat.Type != models.ChatTypePrivate
}

func (t *EventListener) handleNewComment(ctx context.Context, update *models.Update) error {
	// Проверяем, что сообщение от реального пользователя
	if update.Message.From.Username == "" {
		return nil
	}

	tgChannel, post, replyToComment, err := t.getCommentContext(update)
	if err != nil {
		return err
	}
	if tgChannel == nil {
		return nil // Канал не отслеживается
	}

	// --- Медиагруппа: если есть media_group_id, буферизуем ---
	if update.Message.MediaGroupID != "" {
		groupID := update.Message.MediaGroupID
		t.mediaGroupMutex.Lock()
		t.mediaGroupBuffer[groupID] = append(t.mediaGroupBuffer[groupID], update)
		if t.mediaGroupTimers[groupID] == nil {
			t.mediaGroupTimers[groupID] = time.AfterFunc(700*time.Millisecond, func() {
				t.mediaGroupMutex.Lock()
				updates := t.mediaGroupBuffer[groupID]
				delete(t.mediaGroupBuffer, groupID)
				delete(t.mediaGroupTimers, groupID)
				t.mediaGroupMutex.Unlock()
				if len(updates) == 0 {
					return
				}
				first := updates[0]
				// Собираем текст из всех сообщений медиагруппы
				var texts []string
				for _, u := range updates {
					if u.Message.Caption != "" {
						texts = append(texts, u.Message.Caption)
					} else if u.Message.Text != "" {
						texts = append(texts, u.Message.Text)
					}
				}
				caption := strings.Join(texts, "\n")
				attachments := []*entity.Upload{}
				for _, u := range updates {
					a, _ := t.processAttachments(u)
					attachments = append(attachments, a...)
				}
				newComment := &entity.Comment{
					TeamID:            tgChannel.TeamID,
					Platform:          "tg",
					UserPlatformID:    int(first.Message.From.ID),
					CommentPlatformID: first.Message.ID,
					FullName:          fmt.Sprintf("%s %s", first.Message.From.FirstName, first.Message.From.LastName),
					Username:          first.Message.From.Username,
					Text:              caption,
					CreatedAt:         time.Unix(int64(first.Message.Date), 0),
					Attachments:       attachments,
				}
				t.setCommentRelations(newComment, post, replyToComment)
				err := t.setUserAvatar(newComment, first.Message.From.ID)
				if err != nil {
					log.Errorf("Failed to set user avatar: %v", err)
				}
				if newComment.Text == "" && len(newComment.Attachments) == 0 {
					return
				}
				commentID, err := t.commentRepo.AddComment(newComment)
				if err != nil {
					log.Errorf("Failed to save comment: %v", err)
					return
				}
				t.publishCommentEvent(ctx, tgChannel.TeamID, commentID, newComment.PostUnionID, entity.CommentCreated, newComment.CreatedAt)
			})
		}
		t.mediaGroupMutex.Unlock()
		return nil
	}
	// --- Конец медиагруппы ---

	newComment := &entity.Comment{
		TeamID:            tgChannel.TeamID,
		Platform:          "tg",
		UserPlatformID:    int(update.Message.From.ID),
		CommentPlatformID: update.Message.ID,
		FullName:          fmt.Sprintf("%s %s", update.Message.From.FirstName, update.Message.From.LastName),
		Username:          update.Message.From.Username,
		Text:              update.Message.Text,
		CreatedAt:         time.Unix(int64(update.Message.Date), 0),
	}

	// Устанавливаем связи с постом или родительским комментарием
	t.setCommentRelations(newComment, post, replyToComment)

	// Загружаем аватар пользователя
	err = t.setUserAvatar(newComment, update.Message.From.ID)
	if err != nil {
		log.Errorf("Failed to set user avatar: %v", err)
		// Не критичная ошибка, продолжаем
	}

	// Обрабатываем вложения
	err = t.setCommentAttachments(newComment, update)
	if err != nil {
		log.Errorf("Failed to process attachments: %v", err)
		newComment.Text += "\n\n[⚠️ Пользователь прикрепил к комментарию файлы, но не удалось их обработать]"
		newComment.Text = strings.TrimSpace(newComment.Text)
	}

	// Проверяем, что комментарий не пустой
	if newComment.Text == "" && len(newComment.Attachments) == 0 {
		return nil
	}

	// Сохраняем комментарий
	commentID, err := t.commentRepo.AddComment(newComment)
	if err != nil {
		log.Errorf("Failed to save comment: %v", err)
		return err
	}

	// Уведомляем подписчиков
	return t.publishCommentEvent(ctx, tgChannel.TeamID, commentID, newComment.PostUnionID, entity.CommentCreated, newComment.CreatedAt)
}

func (t *EventListener) handleCommentEdit(ctx context.Context, update *models.Update) error {
	log.Debugf("Received edited message: %s", update.EditedMessage.Text)

	// Ищем существующий комментарий
	existingComment, err := t.commentRepo.GetCommentByPlatformID(update.EditedMessage.ID, "tg")
	if errors.Is(err, repo.ErrCommentNotFound) {
		return nil
	}
	if err != nil {
		log.Errorf("Failed to get comment: %v", err)
		return err
	}

	// Обновляем текст
	existingComment.Text = update.EditedMessage.Text

	// Получаем контекст для обновления связей
	update.Message = update.EditedMessage // Унифицируем для использования существующих методов
	_, _, replyToComment, err := t.getCommentContext(update)
	if err == nil && replyToComment != nil {
		existingComment.ReplyToCommentID = replyToComment.ID
		existingComment.PostUnionID = replyToComment.PostUnionID
	}

	// Обрабатываем вложения
	err = t.setCommentAttachments(existingComment, update)
	if err != nil {
		log.Errorf("Failed to process attachments: %v", err)
		existingComment.Text += "\n\n[⚠️ Пользователь прикрепил к комментарию файлы, но не удалось их обработать]"
		existingComment.Text = strings.TrimSpace(existingComment.Text)
	}

	// Проверяем, что комментарий не пустой
	if existingComment.Text == "" && len(existingComment.Attachments) == 0 {
		return nil
	}

	// Сохраняем изменения
	err = t.commentRepo.EditComment(existingComment)
	if err != nil {
		log.Errorf("Failed to update comment: %v", err)
		return err
	}

	// Получаем team ID
	tgChannel, err := t.teamRepo.GetTGChannelByDiscussionId(int(update.EditedMessage.Chat.ID))
	if err != nil {
		log.Errorf("Failed to get team ID: %v", err)
		return err
	}

	// Уведомляем подписчиков
	return t.publishCommentEvent(ctx, tgChannel.TeamID, existingComment.ID, existingComment.PostUnionID, entity.CommentEdited, existingComment.CreatedAt)
}

func (t *EventListener) getCommentContext(update *models.Update) (*entity.TGChannel, *entity.PostPlatform, *entity.Comment, error) {
	discussionID := int(update.Message.Chat.ID)

	tgChannel, err := t.teamRepo.GetTGChannelByDiscussionId(discussionID)
	if errors.Is(err, repo.ErrTGChannelNotFound) {
		return nil, nil, nil, nil
	}
	if err != nil {
		log.Errorf("Failed to get team ID by discussion ID: %v", err)
		return nil, nil, nil, err
	}

	var post *entity.PostPlatform
	var replyToComment *entity.Comment

	if update.Message.ReplyToMessage != nil {
		post, replyToComment, err = t.resolveReplyTarget(update.Message.ReplyToMessage, tgChannel)
		if err != nil {
			return tgChannel, nil, nil, err
		}
	}

	return tgChannel, post, replyToComment, nil
}

func (t *EventListener) resolveReplyTarget(replyMsg *models.Message, tgChannel *entity.TGChannel) (*entity.PostPlatform, *entity.Comment, error) {
	// Случай 1: Ответ на пересланное сообщение из канала
	if replyMsg.ForwardOrigin != nil && replyMsg.ForwardOrigin.MessageOriginChannel != nil {
		post, err := t.postRepo.GetPostPlatformByPost(
			replyMsg.ForwardOrigin.MessageOriginChannel.MessageID,
			tgChannel.ID,
			"tg",
		)
		if errors.Is(err, repo.ErrPostPlatformNotFound) {
			// Возможно это ответ на комментарий
			replyToComment, err := t.commentRepo.GetCommentByPlatformID(replyMsg.ID, "tg")
			if errors.Is(err, repo.ErrCommentNotFound) {
				log.Debugf("Reply target not found as post or comment, ignoring")
				return nil, nil, nil
			}
			return nil, replyToComment, err
		}
		return post, nil, err
	}

	// Случай 2: Ответ на сообщение в обсуждении
	log.Debugf("Received direct reply to comment: %s", replyMsg.Text)
	replyToComment, err := t.commentRepo.GetCommentByPlatformID(replyMsg.ID, "tg")
	if errors.Is(err, repo.ErrCommentNotFound) {
		log.Debugf("Reply target not found as comment, treating as regular comment")
		return nil, nil, nil
	}
	return nil, replyToComment, err
}

func (t *EventListener) setCommentRelations(comment *entity.Comment, post *entity.PostPlatform, replyToComment *entity.Comment) {
	if replyToComment != nil {
		comment.ReplyToCommentID = replyToComment.ID
		comment.PostUnionID = replyToComment.PostUnionID
		comment.PostPlatformID = replyToComment.PostPlatformID
	} else if post != nil {
		comment.PostUnionID = &post.PostUnionId
		comment.PostPlatformID = &post.PostId
	}
}

func (t *EventListener) setUserAvatar(comment *entity.Comment, userID int64) error {
	photos, err := t.bot.GetUserProfilePhotos(t.ctx, &bot.GetUserProfilePhotosParams{
		UserID: userID,
		Limit:  1,
	})
	if err != nil {
		return fmt.Errorf("failed to get user profile photos: %w", err)
	}

	if len(photos.Photos) == 0 {
		return nil // Нет фото профиля
	}

	uploadFileId, err := t.saveFile(photos.Photos[0][0].FileID, "photo")
	if err != nil {
		return fmt.Errorf("failed to save user profile photo: %w", err)
	}

	upload, err := t.uploadUseCase.GetUpload(uploadFileId)
	if err != nil {
		return fmt.Errorf("failed to get uploaded avatar file: %w", err)
	}

	comment.AvatarMediaFile = upload
	return nil
}

func (t *EventListener) setCommentAttachments(comment *entity.Comment, update *models.Update) error {
	attachments, err := t.processAttachments(update)
	if err != nil {
		return err
	}
	comment.Attachments = attachments
	return nil
}

func (t *EventListener) publishCommentEvent(ctx context.Context, teamID, commentID int, postUnionID *int, eventType entity.CommentEventType, occurredAt time.Time) error {
	postID := 0
	if postUnionID != nil {
		postID = *postUnionID
	}

	event := &entity.CommentEvent{
		EventID:    fmt.Sprintf("tg-%d-%d", teamID, commentID),
		TeamID:     teamID,
		PostID:     postID,
		Type:       eventType,
		CommentID:  commentID,
		OccurredAt: occurredAt,
	}

	err := t.eventRepo.PublishCommentEvent(ctx, event)
	if err != nil {
		log.Errorf("Failed to publish comment event: %v", err)
	}
	return err
}
