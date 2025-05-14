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
	Start  time.Time `query:"start"`
	End    time.Time `query:"end"`
}

type GetPostUnionStatsRequest struct {
	UserID      int `query:"-"`
	TeamID      int `query:"team_id"`
	PostUnionID int `query:"post_union_id"`
}

type GetUsersKPIRequest struct {
	UserID int       `query:"-"`
	TeamID int       `query:"team_id"`
	Start  time.Time `query:"start"`
	End    time.Time `query:"end"`
}

type UserKPI struct {
	UserID    int     `json:"user_id"`
	KPI       float64 `json:"kpi"`
	Views     int     `json:"views" db:"views"`
	Comments  int     `json:"comments" db:"comments"`
	Reactions int     `json:"reactions" db:"reactions"`
}

type UsersKPIResponse struct {
	Users []UserKPI `json:"users"`
}

type PostPlatformStats struct {
	ID          int        `json:"-" db:"id"`
	TeamID      int        `json:"team_id" db:"team_id"`
	PostUnionID int        `json:"post_union_id" db:"post_union_id"`
	PeriodStart time.Time  `json:"-" db:"period_start"`
	PeriodEnd   *time.Time `json:"-" db:"period_end"`
	Platform    string     `json:"platform" db:"platform"`
	Views       int        `json:"views" db:"views"`
	Comments    int        `json:"comments"` // В базе данных прямо не хранится, надо считать из смежных таблиц
	Reactions   int        `json:"reactions" db:"reactions"`
}

type PlatformStats struct {
	Views     int `json:"views"`
	Comments  int `json:"comments"`
	Reactions int `json:"reactions"`
}

type PostStats struct {
	PostUnionID int            `json:"post_union_id"`
	Telegram    *PlatformStats `json:"telegram,omitempty"`
	Vkontakte   *PlatformStats `json:"vkontakte,omitempty"`
	// todo: другие платформы
}

type StatsResponse struct {
	Posts []*PostStats `json:"posts"`
}
