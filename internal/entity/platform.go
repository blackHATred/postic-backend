package entity

type Platform string

const (
	Telegram Platform = "tg"
	VK       Platform = "vk"
)

type VKChannel struct {
	ID      int    `db:"id"`
	UserID  int    `db:"user_id"`
	GroupID int    `db:"group_id"`
	APIKey  string `db:"api_key"`
}

type TGChannel struct {
	ID           int `db:"id"`
	UserID       int `db:"user_id"`
	ChannelID    int `db:"channel_id"`
	DiscussionID int `db:"discussion_id"`
}
