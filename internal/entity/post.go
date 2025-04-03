package entity

import (
	"time"
)

type GetPostRequest struct {
	UserID      int
	TeamID      int
	PostUnionID int `json:"post_union_id"`
}

type GetPostsRequest struct {
	UserID int
	TeamID int
	Offset int `json:"offset"`
	Limit  int `json:"-"`
}

type AddPostRequest struct {
	UserID int
	TeamID int
	Text   string `json:"text"`
	// PubTime указывается в UNIX timestamp UTC +0
	PubTime     int      `json:"pub_time"`
	Attachments []int    `json:"attachments"`
	Platforms   []string `json:"platforms"`
}

type EditPostRequest struct {
	UserID      int
	TeamID      int
	PostUnionID int    `json:"post_union_id"`
	Text        string `json:"text"`
}

type DeletePostRequest struct {
	UserID      int
	TeamID      int
	PostUnionID int `json:"post_union_id"`
}

type PostUnion struct {
	ID          int        `json:"id" db:"id"`
	Text        string     `json:"text" db:"text"`
	Platforms   []string   `json:"platforms" db:"platforms"`
	PubDate     *time.Time `json:"pub_datetime" db:"pub_datetime"`
	Attachments []*Upload  `json:"attachments" db:"attachments"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UserID      int        `json:"user_id" db:"user_id"`
	TeamID      int        `json:"team_id" db:"team_id"`
}

type DoActionRequest struct {
	UserID      int
	TeamID      int
	PostUnionID int    `db:"post_union_id"`
	Operation   string `db:"op"`
	Platform    string `db:"platform"`
}

type PostAction struct {
	ID          int       `db:"id"`
	PostUnionID int       `db:"post_union_id"`
	Operation   string    `db:"op"`
	Platform    string    `db:"platform"`
	Status      string    `db:"status"`
	ErrMessage  string    `db:"error_message"`
	CreatedAt   time.Time `db:"created_at"`
}

type PostUnionList struct {
	Posts []*PostUnion `json:"posts"`
}

type PostPlatform struct {
	ID          int    `db:"id"`
	PostUnionId int    `db:"post_union_id"`
	PostId      int    `db:"post_id"`
	Platform    string `db:"platform"`
}

type PostStatusRequest struct {
	UserID      int
	TeamID      int
	PostUnionID int `db:"post_union_id"`
}

type PostActionResponse struct {
	PostID     int    `json:"post_id"`
	Platform   string `json:"platform"`
	Status     string `json:"status"`
	ErrMessage string `json:"err_message"`
}
