package usecase

import "postic-backend/internal/entity"

type Comment interface {
	Add(comment *entity.Comment) error
	Edit(comment *entity.Comment) error
	Delete(commentId int) error

	// GetSlice возвращает агрегированные комментарии организации
	GetSlice(organizationID, offset, limit int) ([]entity.Comment, error)
	// StartListener запускает отслеживание комментариев в указанной организации начиная с
	// события lastEvent (можно поставить -1 и тогда будет запущено отслеживание прямо сейчас
	// с пропуском всех ранних событий)
	StartListener(organizationID, lastEvent int) (<-chan entity.Event, error)

	// ReplyComment отвечает на комментарий пользователя от имени сообщества
	// ReplyComment(commentId int, ) error

	// SummarizeByPostURL получает пост по его url и суммаризирует комментарии под постом
	SummarizeByPostURL(url string) (*entity.Summarize, error)
}
