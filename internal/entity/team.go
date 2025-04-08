package entity

import (
	"time"
)

type TeamUserRole struct {
	TeamID int      `json:"team_id" db:"team_id"`
	UserID int      `json:"user_id" db:"user_id"`
	Roles  []string `json:"roles" db:"roles"`
}

type Team struct {
	ID        int             `json:"id" db:"id"`
	Name      string          `json:"name" db:"name"`
	Secret    string          `json:"-" db:"secret"`
	Users     []*TeamUserRole `json:"users"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
}

type TeamPlatforms struct {
	VKGroupID      int `json:"vk_group_id"   db:"group_id"`
	TGChannelID    int `json:"tg_channel_id" db:"channel_id"`
	TGDiscussionID int `json:"tg_discussion_id" db:"discussion_id"`
}

type UpdateRolesRequest struct {
	RequestUserID int      `json:"-" db:"user_id"`
	TeamID        int      `json:"team_id" db:"team_id"`
	UserID        int      `json:"user_id" db:"user_id"`
	Roles         []string `json:"roles" db:"roles"`
}

type KickUserRequest struct {
	RequestUserID int `json:"-" db:"user_id"`
	KickedUserID  int `json:"kicked_user_id" db:"kicked_user_id"`
	TeamID        int `json:"team_id" db:"team_id"`
}

type SetVKRequest struct {
	RequestUserID int    `json:"-" db:"user_id"`
	TeamID        int    `json:"team_id" db:"team_id"`
	GroupID       int    `json:"group_id" db:"group_id"`
	ApiKey        string `json:"api_key" db:"api_key"`
}

type RenameTeamRequest struct {
	RequestUserID int    `json:"-" db:"user_id"`
	TeamID        int    `json:"team_id" db:"team_id"`
	NewName       string `json:"new_name" db:"new_name"`
}

type CreateTeamRequest struct {
	RequestUserID int    `json:"-" db:"user_id"`
	TeamName      string `json:"team_name" db:"team_name"`
}
