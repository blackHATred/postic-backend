package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/labstack/gommon/log"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"postic-backend/pkg/retry"
	"slices"
	"time"
)

type Comment struct {
	bot         *tgbotapi.BotAPI
	commentRepo repo.Comment
	teamRepo    repo.Team
	uploadRepo  repo.Upload
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

	// Подготавливаем сообщение для ответа
	msg := tgbotapi.NewMessage(int64(discussionID), request.Text)
	msg.ReplyToMessageID = originalMsg.CommentPlatformID

	// Отправляем ответ
	sentMsg, err := t.bot.Send(msg)
	if err != nil {
		return 0, err
	}

	// Создаем запись о новом комментарии
	comment := &entity.Comment{
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

	return nil
}
