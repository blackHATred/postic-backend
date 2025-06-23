package usecase

import (
	"errors"
	"postic-backend/internal/entity"
)

type User interface {
	// Register регистрирует нового пользователя и возвращает его идентификатор
	Register(req *entity.RegisterRequest) (int, error)
	// Login авторизует пользователя и возвращает его идентификатор
	Login(email, password string) (int, error)
	// GetUser возвращает пользователя по его идентификатору
	GetUser(userID int) (*entity.UserProfile, error)
	// UpdatePassword обновляет пароль пользователя
	UpdatePassword(userID int, oldPassword, newPassword string) error
	// UpdateProfile обновляет профиль пользователя
	UpdateProfile(userID int, profile *entity.UpdateProfileRequest) error
	// GetVKAuthURL возвращает URL для авторизации через VK
	GetVKAuthURL() string
	// HandleVKCallback обрабатывает ответ от VK после авторизации
	HandleVKCallback(code string) (int, error)
}

var (
	// Ошибки валидации
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidEmail       = errors.New("invalid email format")
	ErrPasswordTooShort   = errors.New("password too short, minimum length is 8 characters")
	ErrPasswordTooLong    = errors.New("password too long, maximum length is 64 characters")
	ErrVKAuthFailed       = errors.New("vk authorization failed")

	// Ошибки аутентификации и авторизации
	ErrUserNotExists      = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")

	// Системные ошибки
	ErrUserInternal = errors.New("internal server error")
)
