package http

import (
	"errors"
	"net/http"
	"postic-backend/internal/delivery/http/utils"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

type User struct {
	userUseCase     usecase.User
	authManager     utils.Auth
	cookieManager   utils.Cookie
	successLoginURL string
	errorLoginURL   string
}

func NewUser(
	userUseCase usecase.User,
	authManager utils.Auth,
	cookieManager utils.Cookie,
	successLoginURL string,
	errorLoginURL string,
) *User {
	return &User{
		userUseCase:     userUseCase,
		authManager:     authManager,
		cookieManager:   cookieManager,
		successLoginURL: successLoginURL,
		errorLoginURL:   errorLoginURL,
	}
}

func (u *User) Configure(server *echo.Group) {
	server.POST("/register", u.Register)
	server.POST("/login", u.Login)
	server.GET("/me", u.Me)
	server.POST("/logout", u.Logout)
	server.PUT("/update/password", u.UpdatePassword)
	server.PUT("/update/profile", u.UpdateProfile)
	server.GET("/profile", u.GetProfile)
	server.GET("/vk/auth", u.VKAuthURL)
	server.GET("/vk/callback", u.VKCallback)
}

func (u *User) Register(c echo.Context) error {
	var registerRequest entity.RegisterRequest
	err := utils.ReadJSON(c, &registerRequest)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}

	// Валидация данных
	if registerRequest.Email == "" || registerRequest.Password == "" || registerRequest.Nickname == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Email, пароль и имя пользователя обязательны для заполнения",
		})
	}

	userID, err := u.userUseCase.Register(&registerRequest)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrEmailAlreadyExists):
			return c.JSON(http.StatusConflict, echo.Map{
				"error": "Пользователь с таким email уже существует",
			})
		case errors.Is(err, usecase.ErrInvalidEmail):
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error": "Некорректный формат email",
			})
		case errors.Is(err, usecase.ErrPasswordTooShort):
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error": "Пароль слишком короткий, минимальная длина - 8 символов",
			})
		case errors.Is(err, usecase.ErrPasswordTooLong):
			return c.JSON(http.StatusBadRequest, echo.Map{
				"error": "Пароль слишком длинный, максимальная длина - 64 символа",
			})
		default:
			c.Logger().Errorf("Ошибка при регистрации пользователя: %v", err)
			return c.JSON(http.StatusInternalServerError, echo.Map{
				"error": "Произошла непредвиденная ошибка",
			})
		}
	}

	token, err := u.authManager.CreateToken(userID)
	if err != nil {
		c.Logger().Errorf("Ошибка при создании токена: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	expires := time.Now().AddDate(1, 0, 0)
	c.SetCookie(u.cookieManager.SetSessionCookie(token, expires))

	return c.JSON(http.StatusOK, echo.Map{
		"user_id": userID,
		"token":   token,
	})
}

func (u *User) Login(c echo.Context) error {
	var loginRequest entity.LoginRequest
	err := utils.ReadJSON(c, &loginRequest)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}

	// Валидация
	if loginRequest.Email == "" || loginRequest.Password == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Email и пароль обязательны для заполнения",
		})
	}

	userID, err := u.userUseCase.Login(loginRequest.Email, loginRequest.Password)
	if err != nil {
		if errors.Is(err, repo.ErrInvalidPassword) || errors.Is(err, repo.ErrUserNotFound) {
			return c.JSON(http.StatusUnauthorized, echo.Map{
				"error": "Неверный email или пароль",
			})
		}

		c.Logger().Errorf("Ошибка при авторизации пользователя: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	token, err := u.authManager.CreateToken(userID)
	if err != nil {
		c.Logger().Errorf("Ошибка при создании токена: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	expires := time.Now().AddDate(1, 0, 0)
	c.SetCookie(u.cookieManager.SetSessionCookie(token, expires))

	return c.JSON(http.StatusOK, echo.Map{
		"user_id": userID,
		"token":   token,
	})
}

func (u *User) Me(c echo.Context) error {
	userID, err := u.authManager.CheckAuthFromContext(c)
	switch {
	case errors.Is(err, utils.ErrUnauthorized):
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	case err != nil:
		c.Logger().Errorf("Ошибка при проверке авторизации пользователя: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	profile, err := u.userUseCase.GetUser(userID)
	if err != nil {
		if errors.Is(err, repo.ErrUserNotFound) {
			return c.JSON(http.StatusUnauthorized, echo.Map{
				"error": "Пользователь не найден",
			})
		}

		c.Logger().Errorf("Ошибка при получении профиля пользователя: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	return c.JSON(http.StatusOK, profile)
}

func (u *User) Logout(c echo.Context) error {
	_, err := u.authManager.CheckAuthFromContext(c)
	switch {
	case errors.Is(err, utils.ErrUnauthorized):
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	case err != nil:
		c.Logger().Errorf("Ошибка при проверке авторизации пользователя: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	c.SetCookie(u.cookieManager.SetSessionCookie("", time.Now().Add(-time.Hour)))
	return c.NoContent(http.StatusOK)
}

func (u *User) UpdatePassword(c echo.Context) error {
	userID, err := u.authManager.CheckAuthFromContext(c)
	switch {
	case errors.Is(err, utils.ErrUnauthorized):
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	case err != nil:
		c.Logger().Errorf("Ошибка при проверке авторизации пользователя: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	var updatePasswordRequest entity.UpdatePasswordRequest
	err = utils.ReadJSON(c, &updatePasswordRequest)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}

	// Валидация
	if updatePasswordRequest.OldPassword == "" || updatePasswordRequest.NewPassword == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Старый и новый пароли обязательны для заполнения",
		})
	}

	err = u.userUseCase.UpdatePassword(userID, updatePasswordRequest.OldPassword, updatePasswordRequest.NewPassword)
	if err != nil {
		if errors.Is(err, repo.ErrInvalidPassword) {
			return c.JSON(http.StatusUnauthorized, echo.Map{
				"error": "Неверный старый пароль",
			})
		}

		c.Logger().Errorf("Ошибка при обновлении пароля пользователя: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	return c.NoContent(http.StatusOK)
}

func (u *User) UpdateProfile(c echo.Context) error {
	userID, err := u.authManager.CheckAuthFromContext(c)
	switch {
	case errors.Is(err, utils.ErrUnauthorized):
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	case err != nil:
		c.Logger().Errorf("Ошибка при проверке авторизации пользователя: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	var updateProfileRequest entity.UpdateProfileRequest
	err = utils.ReadJSON(c, &updateProfileRequest)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}

	// Валидация
	if updateProfileRequest.Nickname == "" && updateProfileRequest.Email == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Должно быть указано хотя бы одно поле для обновления",
		})
	}

	err = u.userUseCase.UpdateProfile(userID, &updateProfileRequest)
	if err != nil {
		if errors.Is(err, repo.ErrEmailExists) {
			return c.JSON(http.StatusConflict, echo.Map{
				"error": "Пользователь с таким email уже существует",
			})
		}

		c.Logger().Errorf("Ошибка при обновлении профиля пользователя: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	return c.NoContent(http.StatusOK)
}

func (u *User) GetProfile(c echo.Context) error {
	// Получаем user_id из query-параметра
	userIDParam := c.QueryParam("user_id")
	if userIDParam == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Параметр user_id обязателен",
		})
	}

	// Преобразуем строку в int
	userID, err := strconv.Atoi(userIDParam)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Параметр user_id должен быть числом",
		})
	}

	profile, err := u.userUseCase.GetUser(userID)
	if err != nil {
		if errors.Is(err, repo.ErrUserNotFound) {
			return c.JSON(http.StatusNotFound, echo.Map{
				"error": "Пользователь не найден",
			})
		}

		c.Logger().Errorf("Ошибка при получении профиля пользователя: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	return c.JSON(http.StatusOK, profile)
}

func (u *User) VKAuthURL(c echo.Context) error {
	authURL := u.userUseCase.GetVKAuthURL()
	return c.JSON(http.StatusOK, echo.Map{
		"auth_url": authURL,
	})
}

func (u *User) VKCallback(c echo.Context) error {
	code := c.QueryParam("code")
	if code == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Параметры code и redirect_uri обязательны",
		})
	}

	userID, err := u.userUseCase.HandleVKCallback(code)
	if err != nil {
		c.Logger().Errorf("Ошибка при обработке VK callback: %v", err)
		errorMessage := "Произошла непредвиденная ошибка"

		if errors.Is(err, usecase.ErrVKAuthFailed) {
			errorMessage = "Ошибка авторизации через ВКонтакте"
		}

		// Перенаправляем на страницу с ошибкой
		return c.Redirect(http.StatusFound, u.errorLoginURL+"?err="+errorMessage)
	}

	token, err := u.authManager.CreateToken(userID)
	if err != nil {
		c.Logger().Errorf("Ошибка при создании токена: %v", err)
		// Перенаправляем на страницу с ошибкой
		return c.Redirect(http.StatusFound, u.errorLoginURL+"?err="+"Произошла непредвиденная ошибка")
	}

	expires := time.Now().AddDate(1, 0, 0)
	c.SetCookie(u.cookieManager.SetSessionCookie(token, expires))

	return c.Redirect(http.StatusFound, u.successLoginURL)
}
