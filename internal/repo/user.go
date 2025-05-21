package repo

import (
	"errors"
	"postic-backend/internal/entity"
)

type User interface {
	// AddUser добавляет нового пользователя
	AddUser(user *entity.User) (int, error)
	// GetUser возвращает пользователя по его ID
	GetUser(userID int) (*entity.User, error)
	// GetUserByEmail возвращает пользователя по его email
	GetUserByEmail(email string) (*entity.User, error)
	// GetUserByVkID возвращает пользователя по его VK ID
	GetUserByVkID(vkID int) (*entity.User, error)
	// UpdatePassword обновляет пароль пользователя
	UpdatePassword(userID int, passwordHash string) error
	// UpdateProfile обновляет профиль пользователя
	UpdateProfile(userID int, profile *entity.UpdateProfileRequest) error
	// UpdateVkAuth обновляет данные авторизации через ВКонтакте
	UpdateVkAuth(userID, vkID int, accessToken, refreshToken string, expiresAt int64) error
}

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrEmailExists     = errors.New("email already exists")
	ErrInvalidPassword = errors.New("invalid password")
)
