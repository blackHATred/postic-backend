package entity

import "time"

type JustTextComment struct {
	Text string `json:"text" db:"text"`
}

type Comment struct {
	ID                int       `json:"id" db:"id"`
	PostUnionID       int       `json:"post_union_id" db:"post_union_id"`
	Platform          string    `json:"platform" db:"platform"`
	PostPlatformID    int       `json:"post_platform_id" db:"post_platform_id"`
	UserPlatformID    int       `json:"user_platform_id" db:"user_platform_id"`
	CommentPlatformID int       `json:"comment_platform_id" db:"comment_platform_id"`
	FullName          string    `json:"full_name" db:"full_name"`
	Username          string    `json:"username" db:"username"`
	AvatarMediaFile   *Upload   `json:"avatar_mediafile"`
	Text              string    `json:"text" db:"text"`
	ReplyToCommentID  int       `json:"reply_to_comment_id" db:"reply_to_comment_id"`
	IsTeamReply       bool      `json:"is_team_reply" db:"is_team_reply"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	Attachments       []*Upload `json:"attachments"`
}

type SubscribeRequest struct {
	UserID      int `json:"-"`
	TeamID      int `json:"team_id"`
	PostUnionID int `json:"post_union_id"`
}

type GetLastCommentsRequest struct {
	UserID      int       `json:"-"`
	TeamID      int       `json:"team_id"`
	PostUnionID int       `json:"post_union_id"`
	Offset      time.Time `json:"offset"`
	Limit       int       `json:"limit"`
}

type DeleteCommentRequest struct {
	UserID        int  `json:"-"`
	TeamID        int  `json:"team_id"`
	PostCommentID int  `json:"post_comment_id"`
	BanUser       bool `json:"ban_user"`
}

type ReplyCommentRequest struct {
	UserID      int       `json:"-"`
	TeamID      int       `json:"team_id"`
	CommentID   int       `json:"comment_id"`
	Text        string    `json:"text"`
	Attachments []*Upload `json:"attachments"`
}

type SummarizeCommentRequest struct {
	UserID      int `json:"-"`
	TeamID      int `json:"team_id"`
	PostUnionID int `json:"post_union_id"`
}
