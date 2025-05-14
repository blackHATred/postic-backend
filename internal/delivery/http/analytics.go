package http

import (
	"errors"
	"github.com/labstack/echo/v4"
	"net/http"
	"postic-backend/internal/delivery/http/utils"
	"postic-backend/internal/entity"
	"postic-backend/internal/usecase"
)

type Analytics struct {
	analyzeUseCase usecase.Analytics
	authManager    utils.Auth
}

func NewAnalytics(analyzeUseCase usecase.Analytics, authManager utils.Auth) *Analytics {
	return &Analytics{
		analyzeUseCase: analyzeUseCase,
		authManager:    authManager,
	}
}

func (a *Analytics) Configure(server *echo.Group) {
	server.GET("/stats", a.GetStats)
	server.GET("/stats/post", a.GetPostUnionStats)
}

func (a *Analytics) GetStats(c echo.Context) error {
	userID, err := a.authManager.CheckAuthFromContext(c)
	if err != nil {
		return err
	}

	request := &entity.GetStatsRequest{}
	err = utils.ReadQuery(c, request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID

	stats, err := a.analyzeUseCase.GetStats(request)
	switch {
	case errors.Is(err, usecase.ErrUserForbidden):
		return c.JSON(http.StatusForbidden, echo.Map{
			"error": "У вас нет прав для просмотра статистики этой команды",
		})
	case err != nil:
		c.Logger().Error(err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Ошибка сервера",
		})
	}

	return c.JSON(http.StatusOK, stats)
}

func (a *Analytics) GetPostUnionStats(c echo.Context) error {
	userID, err := a.authManager.CheckAuthFromContext(c)
	if err != nil {
		return err
	}

	request := &entity.GetPostUnionStatsRequest{}
	err = utils.ReadQuery(c, request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID

	stats, err := a.analyzeUseCase.GetPostUnionStats(request)
	switch {
	case errors.Is(err, usecase.ErrUserForbidden):
		return c.JSON(http.StatusForbidden, echo.Map{
			"error": "У вас нет прав для просмотра статистики этой команды",
		})
	case err != nil:
		c.Logger().Error(err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Ошибка сервера",
		})
	}

	return c.JSON(http.StatusOK, stats)
}
