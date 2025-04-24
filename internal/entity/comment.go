package entity

import "time"

type JustTextComment struct {
	Text string `json:"text" db:"text"`
}

type Comment struct {
	ID                int       `json:"id" db:"id"`
	TeamID            int       `json:"team_id" db:"team_id"`
	PostUnionID       *int      `json:"post_union_id" db:"post_union_id"`
	Platform          string    `json:"platform" db:"platform"`
	PostPlatformID    *int      `json:"post_platform_id" db:"post_platform_id"`
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
	MarkedAsTicket    bool      `json:"marked_as_ticket" db:"marked_as_ticket"`
}

type GetCommentsRequest struct {
	UserID         int       `query:"-"`
	TeamID         int       `query:"team_id"`
	PostUnionID    int       `query:"post_union_id"`
	Offset         time.Time `query:"offset"`
	Before         bool      `query:"before"`
	Limit          int       `query:"limit"`
	MarkedAsTicket *bool     `query:"marked_as_ticket"`
}

type DeleteCommentRequest struct {
	UserID        int  `json:"-"`
	TeamID        int  `json:"team_id"`
	PostCommentID int  `json:"post_comment_id"`
	BanUser       bool `json:"ban_user"`
}

type ReplyCommentRequest struct {
	UserID      int    `json:"-"`
	TeamID      int    `json:"team_id"`
	CommentID   int    `json:"comment_id"`
	Text        string `json:"text"`
	Attachments []int  `json:"attachments"`
}

type SummarizeCommentRequest struct {
	UserID      int `query:"-"`
	TeamID      int `query:"team_id"`
	PostUnionID int `query:"post_union_id"`
}

type GetCommentRequest struct {
	UserID    int `query:"-"`
	TeamID    int `query:"team_id"`
	CommentID int `query:"comment_id"`
}

type CommentEvent struct {
	CommentID int    `json:"comment_id"`
	Type      string `json:"type"`
}

type Subscriber struct {
	UserID      int `json:"-"`
	TeamID      int `json:"team_id" query:"team_id"`
	PostUnionID int `json:"post_union_id" query:"post_union_id"`
}

type ReplyIdeasRequest struct {
	UserID    int `query:"-"`
	TeamID    int `query:"team_id"`
	CommentID int `query:"comment_id"`
}

type ReplyIdeasResponse struct {
	Ideas []string `json:"ideas"`
}

type MarkAsTicketRequest struct {
	UserID         int  `json:"-"`
	TeamID         int  `json:"team_id"`
	PostCommentID  int  `json:"comment_id"`
	MarkedAsTicket bool `json:"marked_as_ticket"`
}
