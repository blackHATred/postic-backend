package telegram

import (
	"errors"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/labstack/gommon/log"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"postic-backend/pkg/retry"
	"slices"
	"sync"
	"time"
)

type Comment struct {
	bot         *tgbotapi.BotAPI
	commentRepo repo.Comment
	teamRepo    repo.Team
	uploadRepo  repo.Upload
	subscribers map[entity.Subscriber]chan *entity.CommentEvent
	mu          sync.Mutex // Add this line for thread safety
}

// NewTelegramComment создает новый экземпляр обработчика комментариев Telegram
func NewTelegramComment(
	token string,
	commentRepo repo.Comment,
	teamRepo repo.Team,
	uploadRepo repo.Upload,
) (usecase.CommentActionPlatform, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &Comment{
		bot:         bot,
		commentRepo: commentRepo,
		teamRepo:    teamRepo,
		uploadRepo:  uploadRepo,
		subscribers: make(map[entity.Subscriber]chan *entity.CommentEvent),
	}, nil
}

// ReplyComment отправляет комментарий в ответ на другой комментарий от имени группы
func (t *Comment) ReplyComment(request *entity.ReplyCommentRequest) (int, error) {
	// Проверяем, что пользователь является членом команды и имеет права админа или comments
	roles, err := t.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return 0, err
	}
	if !slices.Contains(roles, repo.AdminRole) && !slices.Contains(roles, repo.CommentsRole) {
		return 0, usecase.ErrUserForbidden
	}

	// Получаем информацию о канале дискуссий
	teamID := request.TeamID
	_, discussionID, err := t.teamRepo.GetTGChannelByTeamID(teamID)
	if err != nil {
		return 0, err
	}
	if discussionID == 0 {
		return 0, usecase.ErrReplyCommentUnavailable
	}

	// Получаем оригинальное сообщение
	originalMsg, err := t.commentRepo.GetCommentInfo(request.CommentID)
	if err != nil {
		return 0, err
	}

	// Переменная для хранения отправленного сообщения
	var sentMsg tgbotapi.Message
	chatID := int64(discussionID)

	// Проверяем наличие вложений
	if len(request.Attachments) > 0 {
		// Обрабатываем вложения
		var mediaGroup []interface{}

		for i, attachmentID := range request.Attachments {
			// Получаем информацию о файле
			fileInfo, err := t.uploadRepo.GetUpload(attachmentID)
			if err != nil {
				return 0, err
			}

			// Создаем медиафайл на основе типа
			var inputMedia interface{}

			switch fileInfo.FileType {
			case "photo":
				photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FileReader{
					Name:   fileInfo.FilePath,
					Reader: fileInfo.RawBytes,
				})
				// Добавляем текст только к первому вложению
				if i == 0 && request.Text != "" {
					photo.Caption = request.Text
				}
				inputMedia = photo
			case "video":
				video := tgbotapi.NewInputMediaVideo(tgbotapi.FileReader{
					Name:   fileInfo.FilePath,
					Reader: fileInfo.RawBytes,
				})
				if i == 0 && request.Text != "" {
					video.Caption = request.Text
				}
				inputMedia = video
			case "audio", "voice":
				audio := tgbotapi.NewInputMediaAudio(tgbotapi.FileReader{
					Name:   fileInfo.FilePath,
					Reader: fileInfo.RawBytes,
				})
				if i == 0 && request.Text != "" {
					audio.Caption = request.Text
				}
				inputMedia = audio
			default:
				doc := tgbotapi.NewInputMediaDocument(tgbotapi.FileReader{
					Name:   fileInfo.FilePath,
					Reader: fileInfo.RawBytes,
				})
				if i == 0 && request.Text != "" {
					doc.Caption = request.Text
				}
				inputMedia = doc
			}

			mediaGroup = append(mediaGroup, inputMedia)
		}

		// Отправляем медиагруппу
		mediaMsg := tgbotapi.NewMediaGroup(chatID, mediaGroup)
		mediaMsg.ReplyToMessageID = originalMsg.CommentPlatformID

		sentMessages, err := t.bot.SendMediaGroup(mediaMsg)
		if err != nil {
			return 0, err
		}

		// Берем первое сообщение из группы как основное для дальнейшей обработки
		if len(sentMessages) > 0 {
			sentMsg = sentMessages[0]
		}
	} else {
		// Если вложений нет, отправляем обычное текстовое сообщение
		msg := tgbotapi.NewMessage(chatID, request.Text)
		msg.ReplyToMessageID = originalMsg.CommentPlatformID

		sent, err := t.bot.Send(msg)
		if err != nil {
			return 0, err
		}
		sentMsg = sent
	}

	// Проверяем, был ли установлен reply в отправленном сообщении
	// Если ReplyToMessage == nil, значит оригинальное сообщение было удалено
	// Тогда нужно удалить комментарий в БД и только что отправленное сообщение
	if sentMsg.ReplyToMessage == nil && originalMsg.CommentPlatformID != 0 {
		log.Errorf("Оригинальное сообщение было удалено, удаляем комментарий в БД")
		err = retry.Retry(func() error {
			return t.commentRepo.DeleteComment(request.CommentID)
		})
		if err != nil {
			log.Errorf("Не удалось удалить комментарий в БД: %v", err)
		} else {
			log.Infof("Комментарий с ID %d успешно удален из БД", request.CommentID)
		}

		// Удаляем только что отправленное сообщение
		deleteMsg := tgbotapi.NewDeleteMessage(int64(discussionID), sentMsg.MessageID)
		err = retry.Retry(func() error {
			_, err := t.bot.Request(deleteMsg)
			return err
		})
		if err != nil {
			log.Errorf("Не удалось удалить сообщение в Telegram: %v", err)
		} else {
			log.Infof("Сообщение с ID %d успешно удалено из Telegram", sentMsg.MessageID)
		}

		return 0, usecase.ErrReplyCommentUnavailable
	}

	// Создаем запись о новом комментарии
	comment := &entity.Comment{
		TeamID:            request.TeamID,
		PostUnionID:       originalMsg.PostUnionID,
		Platform:          "tg",
		PostPlatformID:    originalMsg.PostPlatformID,
		UserPlatformID:    0, // 0 означает, что комментарий от имени бота/группы
		CommentPlatformID: sentMsg.MessageID,
		FullName:          "Ответ от имени команды",
		Username:          "",
		Text:              request.Text,
		IsTeamReply:       true,
		ReplyToCommentID:  request.CommentID,
		CreatedAt:         time.Now(),
		Attachments:       make([]*entity.Upload, 0),
	}

	// Добавляем вложения к комментарию
	for _, attachmentID := range request.Attachments {
		upload, err := t.uploadRepo.GetUploadInfo(attachmentID)
		if err != nil {
			log.Errorf("Failed to get attachment info: %v", err)
			continue
		}
		comment.Attachments = append(comment.Attachments, upload)
	}

	// Сохраняем комментарий в БД
	commentID, err := t.commentRepo.AddComment(comment)
	if err != nil {
		// Логируем ошибку, но не прерываем выполнение
		log.Errorf("Failed to save team reply comment: %v", err)
		return 0, err
	}

	return commentID, nil
}

// DeleteComment удаляет комментарий
func (t *Comment) DeleteComment(request *entity.DeleteCommentRequest) error {
	// Получаем информацию о комментарии
	comment, err := t.commentRepo.GetCommentInfo(request.PostCommentID)
	if err != nil {
		return err
	}

	// Получаем информацию о канале дискуссий
	_, discussionID, err := t.teamRepo.GetTGChannelByTeamID(request.TeamID)
	if err != nil {
		return err
	}

	// Создаем запрос на удаление сообщения
	deleteMsg := tgbotapi.NewDeleteMessage(int64(discussionID), comment.CommentPlatformID)

	// Пробуем удалить сообщение с повторами в случае ошибки
	err = retry.Retry(func() error {
		_, err := t.bot.Request(deleteMsg)
		return err
	})

	if err != nil {
		return err
	}

	// Помечаем комментарий как удаленный в БД или удаляем его
	err = t.commentRepo.DeleteComment(request.PostCommentID)
	if err != nil {
		log.Errorf("Failed to mark comment as deleted: %v", err)
	}

	// Если нужно забанить пользователя в telegram, то баним его
	if request.BanUser {
		// Создаем запрос на бан пользователя
		banConfig := tgbotapi.BanChatMemberConfig{
			ChatMemberConfig: tgbotapi.ChatMemberConfig{
				ChatID: int64(discussionID),
				UserID: int64(request.UserID),
			},
			UntilDate:      0, // 0 означает бан навсегда
			RevokeMessages: false,
		}
		// Пробуем забанить пользователя с повторами в случае ошибки
		err = retry.Retry(func() error {
			_, err := t.bot.Request(banConfig)
			return err
		})

		if err != nil {
			log.Errorf("Не удалось забанить пользователя в Telegram: %v", err)
		} else {
			log.Infof("Пользователь с ID %d успешно забанен в канале %d", comment.UserPlatformID, discussionID)
		}
	}

	// уведомляем подписчиков об удалении комментария
	err = t.notifySubscribers(request.PostCommentID, 0, discussionID, "deleted")
	if err != nil {
		log.Errorf("Failed to notify subscribers about deleted comment: %v", err)
	}

	return nil
}

func (t *Comment) SubscribeToCommentEvents(userID, teamID, postUnionID int) <-chan *entity.CommentEvent {
	sub := entity.Subscriber{
		UserID:      userID,
		TeamID:      teamID,
		PostUnionID: postUnionID,
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if ch, ok := t.subscribers[sub]; ok {
		return ch
	}

	ch := make(chan *entity.CommentEvent)
	t.subscribers[sub] = ch
	return ch
}

func (t *Comment) UnsubscribeFromComments(userID, teamID, postUnionID int) {
	sub := entity.Subscriber{
		UserID:      userID,
		TeamID:      teamID,
		PostUnionID: postUnionID,
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if ch, ok := t.subscribers[sub]; ok {
		close(ch)
		delete(t.subscribers, sub)
	}
}

func (t *Comment) notifySubscribers(commentID, postUnionID, discussionID int, eventType string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Определяем, какой команде принадлежит комментарий
	teamId, err := t.teamRepo.GetTeamIDByTGDiscussionID(discussionID)
	if errors.Is(err, repo.ErrTGChannelNotFound) {
		// Если не нашли команду, то пропускаем
		return nil
	}
	if err != nil {
		log.Errorf("Failed to get teamId by postUnionID: %v", err)
		return err
	}
	// Смотрим, какие участники есть в команде
	teamMemberIDs, err := t.teamRepo.GetTeamUsers(teamId)
	if err != nil {
		log.Errorf("Failed to get team members: %v", err)
		return err
	}

	for _, memberID := range teamMemberIDs {
		sub := entity.Subscriber{
			UserID:      memberID,
			TeamID:      teamId,
			PostUnionID: 0,
		}
		if ch, ok := t.subscribers[sub]; ok {
			go func() {
				ch <- &entity.CommentEvent{
					CommentID: commentID,
					Type:      eventType,
				}
			}()
		}
		// также возможен вариант, если подписка осуществлена под конкретный пост
		if postUnionID != 0 {
			sub.PostUnionID = postUnionID
			if ch, ok := t.subscribers[sub]; ok {
				go func() {
					ch <- &entity.CommentEvent{
						CommentID: commentID,
						Type:      eventType,
					}
				}()
			}
		}
	}
	return nil
}
