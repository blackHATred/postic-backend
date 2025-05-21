package usecase

import "postic-backend/internal/entity"

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
}
