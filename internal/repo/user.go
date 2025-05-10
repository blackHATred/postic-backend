package repo

import (
	"errors"
	"postic-backend/internal/entity"
)

type User interface {
	// AddUser добавляет нового пользователя
	AddUser() (int, error)
	// GetUser возвращает пользователя по его ID
	GetUser(userID int) (*entity.User, error)
}

var (
	ErrUserNotFound = errors.New("user not found")
)
