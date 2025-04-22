package repo

import (
	"errors"
	"postic-backend/internal/entity"
	"time"
)

type Comment interface {
	// GetLastComments возвращает последние комментарии к посту со всех платформ
	GetLastComments(postUnionID int, limit int) ([]*entity.JustTextComment, error)
	// AddComment добавляет комментарий к посту
	AddComment(comment *entity.Comment) (int, error)
	// EditComment редактирует комментарий
	EditComment(comment *entity.Comment) error
	// GetComments возвращает комментарии к посту
	GetComments(postUnionID int, offset time.Time, before bool, limit int, markedAsTicket *bool) ([]*entity.Comment, error)
	// GetCommentInfo возвращает информацию о комментарии
	GetCommentInfo(commentID int) (*entity.Comment, error)
	// GetCommentInfoByPlatformID возвращает информацию о комментарии по ID платформы
	GetCommentInfoByPlatformID(platformID int, platform string) (*entity.Comment, error)
	// DeleteComment удаляет комментарий
	DeleteComment(commentID int) error
}

var (
	ErrCommentNotFound = errors.New("comment not found")
)
