package entity

import "time"

type AddPostRequest struct {
	UserId int
	Text   string `json:"text"`
	// PubTime указывается в UNIX timestamp
	PubTime     int      `json:"pub_time"`
	Attachments []int    `json:"attachments"`
	Platforms   []string `json:"platforms"`
}

type PostUnion struct {
	ID          int       `db:"id"`
	Text        string    `db:"text"`
	Platforms   []string  `db:"platforms"`
	PubDate     time.Time `db:"pub_datetime"`
	Attachments []int     `db:"attachments"`
	CreatedAt   time.Time `db:"created_at"`
	UserID      int       `db:"user_id"`
}

type PostAction struct {
	ID          int
	PostUnionID int
	Platform    string
	Status      string
	ErrMessage  string
	CreatedAt   time.Time
}

type GetPostsResponse struct {
	Posts []PostUnion `json:"posts"`
}

type GetPostStatusResponse struct {
	PostID     int    `json:"post_id"`
	Platform   string `json:"platform"`
	Status     string `json:"status"`
	ErrMessage string `json:"err_message"`
}
