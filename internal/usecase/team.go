package usecase

import (
	"errors"
	"postic-backend/internal/entity"
)

type Team interface {
	// GetUserTeams возвращает список команд пользователя
	GetUserTeams(userID int) ([]*entity.Team, error)
	// GetTeamSecret возвращает секрет команды, если запрашивающий секрет пользователь имеет доступ
	GetTeamSecret(userID, teamID int) (string, error)
	// CreateTeam создает команду
	CreateTeam(request *entity.CreateTeamRequest) (int, error)
	// UpdateRoles обновляет роли пользователей в команде
	UpdateRoles(request *entity.UpdateRolesRequest) error
	// RenameTeam переименовывает команду
	RenameTeam(request *entity.RenameTeamRequest) error
	// InviteUser приглашает пользователя в команду
	InviteUser(request *entity.UpdateRolesRequest) error
	// Kick удаляет пользователя из команды
	Kick(request *entity.KickUserRequest) error
	// Platforms возвращает список платформ, привязанных к команде
	Platforms(userID, teamID int) (*entity.TeamPlatforms, error)
	// SetVK устанавливает креды для использования VK API
	SetVK(request *entity.SetVKRequest) error
}

var (
	ErrRoleDoesNotExist     = errors.New("role does not exist")
	ErrTeamNotFound         = errors.New("team not found")
	ErrUserNotFound         = errors.New("user not found")
	ErrUserForbidden        = errors.New("forbidden")
	ErrTeamNameLenIncorrect = errors.New("team name too long")
)
