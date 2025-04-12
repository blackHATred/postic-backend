package usecase

import (
	"errors"
	"postic-backend/internal/entity"
)

type EventSubscriber interface {
	// SubscribeToCommentEvents подписывается на комментарии к посту и возвращает канал, по которому будут
	// приходить ID новых, измененных или удаленных комментариев
	SubscribeToCommentEvents(userID, teamID, postUnionID int) <-chan *entity.CommentEvent
	UnsubscribeFromComments(userID, teamID, postUnionID int)
}

type Listener interface {
	EventSubscriber
	StartListener()
	StopListener()
}

type CommentActionPlatform interface {
	EventSubscriber
	// ReplyComment отправляет комментарий в ответ на другой комментарий от имени группы
	ReplyComment(request *entity.ReplyCommentRequest) (int, error)
	// DeleteComment удаляет комментарий
	DeleteComment(request *entity.DeleteCommentRequest) error
}

type Comment interface {
	// GetComment возвращает комментарий по ID
	GetComment(request *entity.GetCommentRequest) (*entity.Comment, error)
	// GetLastComments возвращает последние комментарии к посту
	GetLastComments(request *entity.GetCommentsRequest) ([]*entity.Comment, error)
	// GetSummarize возвращает сводку по посту
	GetSummarize(request *entity.SummarizeCommentRequest) (*entity.Summarize, error)
	// Subscribe подписывается на получение новых комментариев. Возвращает канал, по которому будут приходить ID
	// новых комментариев
	Subscribe(request *entity.Subscriber) (<-chan *entity.CommentEvent, error)
	// Unsubscribe отписывается от получения новых комментариев
	Unsubscribe(request *entity.Subscriber)
	// ReplyComment отправляет комментарий в ответ на другой комментарий от имени группы
	ReplyComment(request *entity.ReplyCommentRequest) (int, error)
	// DeleteComment удаляет комментарий
	DeleteComment(request *entity.DeleteCommentRequest) error
}

var (
	ErrReplyCommentUnavailable = errors.New("reply comment unavailable")
)
