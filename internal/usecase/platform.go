package usecase

import (
	"postic-backend/internal/entity"
	"time"
)

type Telegram interface {
	// AddPostInQueue добавляет пост в очередь на публикацию
	AddPostInQueue(postAction *entity.PostAction) error
	// GetComments возвращает комментарии к посту с учётом оффсета по времени и лимита
	GetComments(postUnionID int, offset time.Time, limit int) ([]*entity.TelegramComment, error)
	// GetRawAttachment возвращает вложение с заполненным содержимым по его идентификатору
	GetRawAttachment(attachmentID int) (*entity.TelegramMessageAttachment, error)
	// GetUser возвращает пользователя Telegram по его идентификатору
	GetUser(userID int) (*entity.PlatformUser, error)
	// Subscribe возвращает канал для подписки на новые комментарии
	Subscribe() chan *entity.TelegramComment
	// Unsubscribe отписывает канал от новых комментариев
	Unsubscribe(ch chan *entity.TelegramComment)
}

type Vkontakte interface {
	// AddPostInQueue добавляет пост в очередь на публикацию
	AddPostInQueue(postAction entity.PostAction) error
}
