package repo

import (
	"context"
	"postic-backend/internal/entity"
)

type CommentEventRepository interface {
	PublishCommentEvent(ctx context.Context, event *entity.CommentEvent) error
	SubscribeCommentEvents(ctx context.Context, teamID int, postID int) (<-chan *entity.CommentEvent, error)
}
