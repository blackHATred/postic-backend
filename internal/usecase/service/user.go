package service

import (
	"errors"
	"golang.org/x/crypto/bcrypt"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
)

type User struct {
	userRepo repo.User
}

func NewUser(userRepo repo.User) usecase.User {
	return &User{userRepo: userRepo}
}

func (u *User) Register(req *entity.RegisterRequest) (int, error) {
	// Хешируем пароль пользователя
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	user := &entity.User{
		Nickname:     req.Nickname,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
	}

	return u.userRepo.AddUser(user)
}

func (u *User) Login(email, password string) (int, error) {
	user, err := u.userRepo.GetUserByEmail(email)
	if err != nil {
		if errors.Is(err, repo.ErrUserNotFound) {
			return 0, repo.ErrInvalidPassword
		}
		return 0, err
	}

	// Проверяем пароль пользователя (если есть VK авторизация и нет пароля, позволяем войти)
	if user.PasswordHash != "" {
		err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
		if err != nil {
			return 0, repo.ErrInvalidPassword
		}
	} else if user.VkID == "" {
		// Если нет ни пароля, ни авторизации через VK, то не разрешаем вход
		return 0, repo.ErrInvalidPassword
	}

	return user.ID, nil
}

func (u *User) GetUser(userID int) (*entity.UserProfile, error) {
	user, err := u.userRepo.GetUser(userID)
	if err != nil {
		return nil, err
	}

	return &entity.UserProfile{
		ID:       user.ID,
		Nickname: user.Nickname,
		Email:    user.Email,
	}, nil
}

func (u *User) UpdatePassword(userID int, oldPassword, newPassword string) error {
	user, err := u.userRepo.GetUser(userID)
	if err != nil {
		return err
	}

	// Проверяем старый пароль, если он установлен
	if user.PasswordHash != "" {
		err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword))
		if err != nil {
			return repo.ErrInvalidPassword
		}
	}

	// Хешируем новый пароль
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return u.userRepo.UpdatePassword(userID, string(hashedPassword))
}

func (u *User) UpdateProfile(userID int, profile *entity.UpdateProfileRequest) error {
	return u.userRepo.UpdateProfile(userID, profile)
}

func (u *User) SetVK(userID int, vkGroupID int, apiKey string) error {
	// Этот метод остался от старого интерфейса, имплементация будет в будущем
	return nil
}
