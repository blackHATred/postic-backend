package usecase

import (
	"errors"
	"postic-backend/internal/entity"
)

type Listener interface {
	StartListener()
	StopListener()
	// SubscribeToCommentEvents подписывается на комментарии к посту и возвращает канал, по которому будут
	//приходить ID новых, измененных или удаленных комментариев
	SubscribeToCommentEvents(teamId, postUnionId int) <-chan int
	UnsubscribeFromComments(teamId, postUnionId int)
}

type CommentActionPlatform interface {
	// ReplyComment отправляет комментарий в ответ на другой комментарий от имени группы
	ReplyComment(request *entity.ReplyCommentRequest) (int, error)
	// DeleteComment удаляет комментарий
	DeleteComment(request *entity.DeleteCommentRequest) error
}

type Comment interface {
	// GetComment возвращает комментарий по ID
	GetComment(request *entity.GetCommentRequest) (*entity.Comment, error)
	// GetLastComments возвращает последние комментарии к посту
	GetLastComments(request *entity.GetLastCommentsRequest) ([]*entity.Comment, error)
	// GetSummarize возвращает сводку по посту
	GetSummarize(request *entity.SummarizeCommentRequest) (*entity.Summarize, error)
	// Subscribe подписывается на получение новых комментариев. Возвращает канал, по которому будут приходить ID
	// новых комментариев
	Subscribe(request *entity.SubscribeRequest) (<-chan int, error)
	// Unsubscribe отписывается от получения новых комментариев
	Unsubscribe(request *entity.SubscribeRequest)
	// ReplyComment отправляет комментарий в ответ на другой комментарий от имени группы
	ReplyComment(request *entity.ReplyCommentRequest) (int, error)
	// DeleteComment удаляет комментарий
	DeleteComment(request *entity.DeleteCommentRequest) error
}

var (
	ErrReplyCommentUnavailable = errors.New("reply comment unavailable")
)
