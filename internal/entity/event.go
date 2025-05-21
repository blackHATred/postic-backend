package entity

import "time"

type CommentEventType string

const (
	CommentCreated CommentEventType = "created"
	CommentDeleted CommentEventType = "deleted"
	CommentEdited  CommentEventType = "edited"
)

type CommentEvent struct {
	EventID    string           `json:"-" msgpack:"event_id"`
	TeamID     int              `json:"-" msgpack:"team_id"`
	PostID     int              `json:"-" msgpack:"post_id"`
	Type       CommentEventType `json:"type" msgpack:"type"`
	CommentID  int              `json:"comment_id" msgpack:"comment_id"`
	OccurredAt time.Time        `json:"-" msgpack:"occurred_at"`
}
