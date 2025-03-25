package entity

type LoginRequest struct {
	UserID int `json:"user_id"`
}

type SetVKRequest struct {
	GroupID int    `json:"group_id"`
	APIKey  string `json:"api_key"`
}

type SetTGRequest struct {
	UserID       int    `json:"user_id"`
	ChannelID    int    `json:"channel_id"`
	DiscussionID int    `json:"discussion_id"`
	UserSecret   string `json:"user_secret"`
}

type User struct {
	ID     int    `json:"id" db:"id"`
	Secret string `json:"secret" db:"secret"`
}
