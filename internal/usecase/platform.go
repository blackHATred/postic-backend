package usecase

import (
	"postic-backend/internal/entity"
	"time"
)

type Platform interface {
	AddCreatePostInQueue(postAction *entity.AddPostAction) error
	AddEditPostInQueue(postAction *entity.AddPostAction) error
	AddDeletePostInQueue(postAction *entity.AddPostAction) error
	GetComments(postUnionID int, offset time.Time, limit int) ([]*entity.Comment, error)
	CommentsSubscribe()
}

type Telegram interface {
	// AddPostInQueue добавляет пост в очередь на публикацию
	AddPostInQueue(postAction *entity.AddPostAction) error
	// GetComments возвращает комментарии к посту с учётом оффсета по времени и лимита
	GetComments(postUnionID int, offset time.Time, limit int) ([]*entity.TelegramComment, error)
	// GetRawAttachment возвращает вложение с заполненным содержимым по его идентификатору
	GetRawAttachment(attachmentID int) (*entity.TelegramMessageAttachment, error)
	// Subscribe возвращает канал для подписки на новые комментарии
	Subscribe(userID int) chan *entity.TelegramComment
	// Unsubscribe отписывает канал от новых комментариев
	Unsubscribe(ch chan *entity.TelegramComment)
	// GetUserAvatar возвращает аватар пользователя
	GetUserAvatar(userID int) ([]byte, error)
}

type Vkontakte interface {
	// AddPostInQueue добавляет пост в очередь на публикацию
	AddPostInQueue(postAction *entity.AddPostAction) error
}

type Odnoklassniki interface {
}
