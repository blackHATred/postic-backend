package http

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"postic-backend/internal/delivery/http/utils"
	"postic-backend/internal/entity"
	"postic-backend/internal/usecase"
	"time"
)

type User struct {
	userUseCase   usecase.User
	cookieManager *utils.CookieManager
}

func NewUser(userUseCase usecase.User, cookieManager *utils.CookieManager) *User {
	return &User{
		userUseCase:   userUseCase,
		cookieManager: cookieManager,
	}
}

func (u *User) Configure(server *echo.Group) {
	server.POST("/register", u.Register)
	server.POST("/login", u.Login)
	server.PUT("/set/vk", u.SetVK)
}

func (u *User) Register(c echo.Context) error {
	userID, err := u.userUseCase.Register()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}
	// Куки на год
	expires := time.Now().AddDate(1, 0, 0)
	c.SetCookie(u.cookieManager.NewUserIDCookie(userID, expires))
	return c.JSON(http.StatusOK, echo.Map{
		"user_id": userID,
	})
}

func (u *User) Login(c echo.Context) error {
	// Извлекаем из запроса айди, под которым хочет войти юзер. Пароли пока не проверяем
	loginRequest := &entity.LoginRequest{}
	err := utils.ReadJSON(c, &loginRequest)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	userID, err := u.userUseCase.Login(loginRequest.UserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}
	// Куки на год
	expires := time.Now().AddDate(1, 0, 0)
	c.SetCookie(u.cookieManager.NewUserIDCookie(userID, expires))
	return c.JSON(http.StatusOK, echo.Map{
		"user_id": userID,
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
	userID, err := u.cookieManager.GetUserIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}
	err = u.userUseCase.SetVK(userID, vkRequest.GroupID, vkRequest.APIKey)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, echo.Map{
		"message": "Группа ВК установлена",
	})
}
