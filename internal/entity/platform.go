package entity

import "time"

type VKChannel struct {
	ID          int       `db:"id"`
	TeamID      int       `db:"team_id"`
	GroupID     int       `db:"group_id"`
	AdminAPIKey string    `db:"admin_api_key"`
	GroupAPIKey string    `db:"group_api_key"`
	LastUpdated time.Time `db:"last_updated_timestamp"`
}

type TGChannel struct {
	ID           int  `db:"id"`
	TeamID       int  `db:"team_id"`
	ChannelID    int  `db:"channel_id"`
	DiscussionID *int `db:"discussion_id"`
}
