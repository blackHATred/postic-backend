package entity

import "time"

type UpdatePostStatsRequest struct {
	UserID      int `json:"-"`
	TeamID      int `json:"team_id"`
	PostUnionID int `json:"post_union_id"`
}

type GetStatsRequest struct {
	UserID int       `query:"-"`
	TeamID int       `query:"team_id"`
	Offset time.Time `query:"offset"`
	Before bool      `query:"before"`
}

type GetPostUnionStatsRequest struct {
	UserID      int `query:"-"`
	TeamID      int `query:"team_id"`
	PostUnionID int `query:"post_union_id"`
}

type PostPlatformStats struct {
	TeamID      int       `db:"team_id"`
	PostUnionID int       `db:"post_union_id"`
	Platform    string    `db:"platform"`
	Views       int       `db:"views"`
	Comments    int       // в базе данных прямо не хранится, надо считать из смежных таблиц
	Reactions   int       `db:"reactions"`
	LastUpdate  time.Time `db:"last_update"`
}

type PlatformStats struct {
	Views     int `json:"views"`
	Comments  int `json:"comments"`
	Reactions int `json:"reactions"`
}

type PostStats struct {
	PostUnionID int            `json:"post_union_id"`
	Telegram    *PlatformStats `json:"telegram,omitempty"`
	// todo: другие платформы
}

type StatsResponse struct {
	Posts []*PostStats `json:"posts"`
}
