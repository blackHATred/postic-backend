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
	ID          int
	Text        string
	PubDate     time.Time
	Attachments []int
	CreatedAt   time.Time
	UserID      int
}

type PostActionVK struct {
	ID          int
	PostUnionID int
}
