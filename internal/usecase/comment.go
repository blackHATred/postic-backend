package usecase

import (
	"context"
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
	StartListener()
	StopListener()
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
	GetLastComments(request *entity.GetCommentsRequest) ([]*entity.Comment, error)
	// GetSummarize возвращает сводку по посту
	GetSummarize(request *entity.SummarizeCommentRequest) (*entity.Summarize, error)
	// Subscribe подписывается на получение новых комментариев. Теперь принимает context
	Subscribe(ctx context.Context, request *entity.Subscriber) (<-chan *entity.CommentEvent, error)
	// ReplyComment отправляет комментарий в ответ на другой комментарий от имени группы
	ReplyComment(request *entity.ReplyCommentRequest) (int, error)
	// DeleteComment удаляет комментарий
	DeleteComment(request *entity.DeleteCommentRequest) error
	// ReplyIdeas предлагает варианты быстрого ответа на комментарий
	ReplyIdeas(request *entity.ReplyIdeasRequest) (*entity.ReplyIdeasResponse, error)
	// MarkAsTicket помечает комментарий как тикет
	MarkAsTicket(request *entity.MarkAsTicketRequest) error
}

var (
	ErrCommentNotFound          = errors.New("comment not found")
	ErrReplyCommentUnavailable  = errors.New("reply comment unavailable")
	ErrCannotGenerateReplyIdeas = errors.New("cannot generate reply ideas")
)
