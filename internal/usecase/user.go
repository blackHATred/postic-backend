package usecase

import "postic-backend/internal/entity"

type User interface {
	// Register регистрирует нового пользователя и возвращает его идентификатор. Пока что без паролей
	Register() (int, error)
	// Login авторизует пользователя и возвращает его идентификатор. Пока что без паролей
	Login(userID int) (int, error)
	// GetUser возвращает пользователя по его идентификатору
	GetUser(userID int) (*entity.User, error)
	// SetVK устанавливает группу ВКонтакте для пользователя
	SetVK(userID int, vkGroupID int, apiKey string) error
}
