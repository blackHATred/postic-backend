package repo

import (
	"postic-backend/internal/entity"
	"time"
)

type Comment interface {
	// GetLastComments возвращает последние комментарии к посту со всех платформ
	GetLastComments(postUnionID int, limit int) ([]*entity.JustTextComment, error)
	// GetTGComments возвращает комментарии к посту
	GetTGComments(postUnionID int, offset time.Time, limit int) ([]*entity.TelegramComment, error)
	// AddTGComment добавляет комментарий к посту
	AddTGComment(comment *entity.TelegramComment) (int, error)
	// GetTGAttachment возвращает вложение по его идентификатору
	GetTGAttachment(attachmentID int) (*entity.TelegramMessageAttachment, error)
}
