package repo

import (
	"errors"
	"postic-backend/internal/entity"
)

type Team interface {
	// AddTeam добавляет команду и возвращает ID команды
	AddTeam(team *entity.Team) (int, error)
	// EditTeam изменяет команду
	EditTeam(team *entity.Team) error
	// GetTeam возвращает команду по ID
	GetTeam(teamId int) (*entity.Team, error)
	// GetTeamUsers возвращает ID пользователей команды по ID команды
	GetTeamUsers(teamId int) ([]int, error)
	// GetTeamIDBySecret возвращает ID команды по секретному ключу
	GetTeamIDBySecret(secret string) (int, error)
	// GetTeamIDByPostUnionID возвращает ID команды, которая может видеть пост с данным ID
	GetTeamIDByPostUnionID(postUnionID int) (int, error)
	// GetTeamIDByTGDiscussionID возвращает ID команды по ID обсуждения
	GetTeamIDByTGDiscussionID(discussionId int) (int, error)
	GetTeamIDByVKGroupID(groupId int) (int, error)

	// GetUserTeams возвращает список айди команд пользователя
	GetUserTeams(userID int) ([]int, error)
	// GetTeamUserRoles возвращает роли пользователя в команде
	GetTeamUserRoles(teamId int, userId int) ([]string, error)
	// EditTeamUserRoles обновляет роли пользователя в команде. Если их нет, то создает новую запись в бд
	EditTeamUserRoles(teamId int, userId int, roles []string) error
	// DeleteTeamUserRoles удаляет все роли пользователя в команде, по сути исключая его из команды
	DeleteTeamUserRoles(teamId int, userId int) error

	// GetTGChannelByTeamID возвращает ID телеграм канала и ID обсуждений (если их нет, то 0) по ID команды
	GetTGChannelByTeamID(teamId int) (int, int, error)
	// PutTGChannel привязывает ID телеграм канала и ID обсуждения к команде
	PutTGChannel(teamId int, channelId int, discussionId int) error
	// GetTGChannelByDiscussionId возвращает ID телеграм канала (в нашей системе, а не реальный) по ID обсуждения
	GetTGChannelByDiscussionId(discussionId int) (int, error)

	// PutVKGroup привязывает группу к команде
	PutVKGroup(teamId int, groupId int, adminApiKey string, groupApiKey string) error
	// GetVKCredsByTeamID возвращает ID группы и ключи доступа к ней по ID команды
	GetVKCredsByTeamID(teamId int) (int, string, string, error)
}

const (
	AdminRole     = "admin"
	PostsRole     = "posts"
	CommentsRole  = "comments"
	AnalyticsRole = "analytics"
)

var (
	ErrTeamNotFound      = errors.New("team not found")
	ErrTGChannelNotFound = errors.New("tg channel not found")
)
