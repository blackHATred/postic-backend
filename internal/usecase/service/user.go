package service

import (
	"errors"
	"github.com/SevereCloud/vksdk/v3/api"
	"github.com/labstack/gommon/log"
	"golang.org/x/crypto/bcrypt"
	"postic-backend/internal/delivery/http/utils"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"regexp"
	"time"
)

// Регулярное выражение для валидации email
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// Минимальная и максимальная длина пароля
const (
	MinPasswordLength = 8
	MaxPasswordLength = 64
)

type User struct {
	vkAuth   *utils.VKOAuth
	userRepo repo.User
}

func NewUser(userRepo repo.User, vkAuth *utils.VKOAuth) usecase.User {
	return &User{
		userRepo: userRepo,
		vkAuth:   vkAuth,
	}
}

// validateEmail проверяет корректность формата email
func validateEmail(email string) error {
	if !emailRegex.MatchString(email) {
		return usecase.ErrInvalidEmail
	}
	return nil
}

// validatePassword проверяет длину пароля
func validatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return usecase.ErrPasswordTooShort
	}
	if len(password) > MaxPasswordLength {
		return usecase.ErrPasswordTooLong
	}
	return nil
}

func (u *User) Register(req *entity.RegisterRequest) (int, error) {
	// Валидация email
	if err := validateEmail(req.Email); err != nil {
		return 0, err
	}

	// Валидация пароля
	if err := validatePassword(req.Password); err != nil {
		return 0, err
	}

	// Хешируем пароль пользователя
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	password := string(hashedPassword)

	user := &entity.User{
		Nickname:     req.Nickname,
		Email:        &req.Email,
		PasswordHash: &password,
	}

	id, err := u.userRepo.AddUser(user)
	if err != nil {
		if errors.Is(err, repo.ErrEmailExists) {
			return 0, usecase.ErrEmailAlreadyExists
		}
		return 0, err
	}

	return id, err
}

func (u *User) Login(email, password string) (int, error) {
	// Валидация email
	if err := validateEmail(email); err != nil {
		return 0, err
	}

	user, err := u.userRepo.GetUserByEmail(email)
	if err != nil {
		if errors.Is(err, repo.ErrUserNotFound) {
			return 0, repo.ErrInvalidPassword
		}
		return 0, err
	}

	// Проверяем пароль пользователя (если есть VK авторизация и нет пароля, позволяем войти)
	if user.PasswordHash != nil {
		err = bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password))
		if err != nil {
			return 0, repo.ErrInvalidPassword
		}
	} else if user.VkID == nil {
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
	email := ""
	if user.Email != nil {
		email = *user.Email
	}

	return &entity.UserProfile{
		ID:       user.ID,
		Nickname: user.Nickname,
		Email:    email,
	}, nil
}

func (u *User) UpdatePassword(userID int, oldPassword, newPassword string) error {
	// Валидация нового пароля
	if err := validatePassword(newPassword); err != nil {
		return err
	}

	user, err := u.userRepo.GetUser(userID)
	if err != nil {
		return err
	}

	// Проверяем старый пароль, если он установлен
	if user.PasswordHash != nil {
		err = bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(oldPassword))
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
	// Валидация email если он был предоставлен
	if profile.Email != "" {
		if err := validateEmail(profile.Email); err != nil {
			return err
		}
	}

	return u.userRepo.UpdateProfile(userID, profile)
}

func (u *User) GetVKAuthURL() string {
	return u.vkAuth.GetAuthURL("state")
}

func (u *User) HandleVKCallback(code string) (int, error) {
	tok, err := u.vkAuth.Exchange(code)
	if err != nil {
		log.Errorf("Failed to exchange token: %v", err)
		return 0, usecase.ErrVKAuthFailed
	}
	vk := api.NewVK(tok.AccessToken)
	response, err := vk.UsersGet(api.Params{
		"fields": "id,first_name,last_name,email",
	})
	if err != nil {
		log.Errorf("Failed to get user info: %v", err)
		return 0, usecase.ErrVKAuthFailed
	}
	// если пользователя нет, то регистрируем
	// если есть - авторизуемся
	user, err := u.userRepo.GetUserByVkID(response[0].ID)
	if err != nil {
		if errors.Is(err, repo.ErrUserNotFound) {
			// Регистрация нового пользователя
			// expires переводим из int64 в time
			expires := time.Unix(tok.ExpiresIn, 0)
			user = &entity.User{
				Nickname:         response[0].FirstName + " " + response[0].LastName,
				VkID:             &response[0].ID,
				VkAccessToken:    &tok.AccessToken,
				VkRefreshToken:   &tok.RefreshToken,
				VkTokenExpiresAt: &expires,
			}
			id, err := u.userRepo.AddUser(user)
			if err != nil {
				return 0, err
			}
			return id, nil
		}
		return 0, err
	}
	// Обновляем токены
	err = u.userRepo.UpdateVkAuth(user.ID, response[0].ID, tok.AccessToken, tok.RefreshToken, time.Unix(tok.ExpiresIn, 0).Unix())
	if err != nil {
		log.Errorf("Failed to update user tokens: %v", err)
		return 0, usecase.ErrVKAuthFailed
	}
	return user.ID, nil
}
