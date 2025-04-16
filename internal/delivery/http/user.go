package http

import (
	"errors"
	"github.com/labstack/echo/v4"
	"net/http"
	"postic-backend/internal/delivery/http/utils"
	"postic-backend/internal/entity"
	"postic-backend/internal/usecase"
	"time"
)

type User struct {
	userUseCase   usecase.User
	authManager   utils.Auth
	cookieManager utils.Cookie
}

func NewUser(userUseCase usecase.User, authManager utils.Auth, cookieManager utils.Cookie) *User {
	return &User{
		userUseCase:   userUseCase,
		authManager:   authManager,
		cookieManager: cookieManager,
	}
}

func (u *User) Configure(server *echo.Group) {
	server.POST("/register", u.Register)
	server.POST("/login", u.Login)
	server.GET("/me", u.Me)
	server.PUT("/set/vk", u.SetVK)
}

func (u *User) Register(c echo.Context) error {
	userID, err := u.userUseCase.Register()
	if err != nil {
		c.Logger().Errorf("Ошибка при регистрации пользователя: %v", err)
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
	token, err := u.authManager.Login(loginRequest.UserID)
	if err != nil {
		c.Logger().Errorf("Ошибка при авторизации пользователя: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}
	expires := time.Now().AddDate(1, 0, 0)
	c.SetCookie(u.cookieManager.SetSessionCookie(token, expires))
	return c.JSON(http.StatusOK, echo.Map{
		"token": token,
	})
}

func (u *User) Me(c echo.Context) error {
	userId, err := u.authManager.CheckAuthFromContext(c)
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
	return c.JSON(http.StatusOK, echo.Map{
		"user_id": userId,
	})
}

func (u *User) SetVK(c echo.Context) error {
	// Устанавливает группу вк, управляемую пользователем
	vkRequest := &entity.SetVKRequest{}
	err := utils.ReadJSON(c, &vkRequest)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	userId, err := u.authManager.CheckAuthFromContext(c)
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
	err = u.userUseCase.SetVK(userId, vkRequest.GroupID, vkRequest.ApiKey)
	if err != nil {
		c.Logger().Errorf("Ошибка при установке группы ВК: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}
	return c.JSON(http.StatusOK, echo.Map{
		"message": "Группа ВК установлена",
	})
}
