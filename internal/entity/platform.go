package entity

type VKChannel struct {
	ID      int
	UserID  int
	GroupID int
	APIKey  string
}

type TGChannel struct {
	ID           int `db:"id"`
	UserID       int `db:"user_id"`
	ChannelID    int `db:"channel_id"`
	DiscussionID int `db:"discussion_id"`
}
