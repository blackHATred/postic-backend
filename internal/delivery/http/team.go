package http

import (
	"errors"
	"github.com/labstack/echo/v4"
	"net/http"
	"postic-backend/internal/delivery/http/utils"
	"postic-backend/internal/entity"
	"postic-backend/internal/usecase"
	"strconv"
)

type Team struct {
	teamUseCase usecase.Team
	authManager utils.Auth
}

func NewTeam(teamUseCase usecase.Team, authManager utils.Auth) *Team {
	return &Team{
		teamUseCase: teamUseCase,
		authManager: authManager,
	}
}

func (t *Team) Configure(server *echo.Group) {
	server.GET("/my_teams", t.MyTeams)
	server.GET("/secret", t.Secret)
	server.PUT("/rename", t.Rename)
	server.POST("/create", t.Create)
	server.POST("/invite", t.Invite)
	server.PUT("/roles", t.Roles)
	server.POST("/kick", t.Kick)
	server.GET("/platforms", t.Platforms)
	server.PUT("/set_vk", t.SetVK)
}

func (t *Team) MyTeams(c echo.Context) error {
	userID, err := t.authManager.CheckAuthFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Unauthorized",
		})
	}

	teams, err := t.teamUseCase.GetUserTeams(userID)
	if err != nil {
		c.Logger().Errorf("Ошибка при получении команд пользователя: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"teams": teams,
	})
}

func (t *Team) Secret(c echo.Context) error {
	userID, err := t.authManager.CheckAuthFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Unauthorized",
		})
	}

	teamID, err := strconv.Atoi(c.QueryParam("team_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат team_id",
		})
	}

	secret, err := t.teamUseCase.GetTeamSecret(userID, teamID)
	switch {
	case errors.Is(err, usecase.ErrTeamNotFound):
		return c.JSON(http.StatusNotFound, echo.Map{
			"error": "Команда не найдена",
		})
	case errors.Is(err, usecase.ErrUserForbidden):
		return c.JSON(http.StatusForbidden, echo.Map{
			"error": "Для этой операции требуются права администратора",
		})
	case err != nil:
		c.Logger().Errorf("Ошибка при получении секрета команды: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"secret": secret,
	})
}

func (t *Team) Rename(c echo.Context) error {
	userID, err := t.authManager.CheckAuthFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Unauthorized",
		})
	}

	var request entity.RenameTeamRequest
	err = utils.ReadJSON(c, &request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}

	request.RequestUserID = userID

	err = t.teamUseCase.RenameTeam(&request)
	switch {
	case errors.Is(err, usecase.ErrTeamNotFound):
		return c.JSON(http.StatusNotFound, echo.Map{
			"error": "Команда не найдена",
		})
	case errors.Is(err, usecase.ErrUserForbidden):
		return c.JSON(http.StatusForbidden, echo.Map{
			"error": "Для этой операции требуются права администратора",
		})
	case err != nil:
		c.Logger().Errorf("Ошибка при переименовании команды: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	return c.NoContent(http.StatusOK)
}

func (t *Team) Create(c echo.Context) error {
	userID, err := t.authManager.CheckAuthFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Unauthorized",
		})
	}

	var request entity.CreateTeamRequest
	err = utils.ReadJSON(c, &request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}

	request.RequestUserID = userID

	id, err := t.teamUseCase.CreateTeam(&request)
	switch {
	case errors.Is(err, usecase.ErrTeamNameLenIncorrect):
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Имя команды должно быть от 1 до 64 символов",
		})
	case err != nil:
		c.Logger().Errorf("Ошибка при создании команды: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	return c.JSON(http.StatusCreated, echo.Map{
		"team_id": id,
	})
}

func (t *Team) Invite(c echo.Context) error {
	userID, err := t.authManager.CheckAuthFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Unauthorized",
		})
	}

	var request entity.UpdateRolesRequest
	err = utils.ReadJSON(c, &request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}

	request.RequestUserID = userID

	err = t.teamUseCase.InviteUser(&request)
	switch {
	case errors.Is(err, usecase.ErrTeamNotFound):
		return c.JSON(http.StatusNotFound, echo.Map{
			"error": "Команда не найдена",
		})
	case errors.Is(err, usecase.ErrUserNotFound):
		return c.JSON(http.StatusNotFound, echo.Map{
			"error": "Пользователь не найден",
		})
	case errors.Is(err, usecase.ErrUserForbidden):
		return c.JSON(http.StatusForbidden, echo.Map{
			"error": "Для этой операции требуются права администратора",
		})
	case errors.Is(err, usecase.ErrRoleDoesNotExist):
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Указана несуществующая роль",
		})
	case err != nil:
		c.Logger().Errorf("Ошибка при приглашении пользователя в команду: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	return c.NoContent(http.StatusOK)
}

func (t *Team) Roles(c echo.Context) error {
	userID, err := t.authManager.CheckAuthFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Unauthorized",
		})
	}

	var request entity.UpdateRolesRequest
	err = utils.ReadJSON(c, &request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}

	request.RequestUserID = userID

	err = t.teamUseCase.UpdateRoles(&request)
	switch {
	case errors.Is(err, usecase.ErrTeamNotFound):
		return c.JSON(http.StatusNotFound, echo.Map{
			"error": "Команда не найдена",
		})
	case errors.Is(err, usecase.ErrUserNotFound):
		return c.JSON(http.StatusNotFound, echo.Map{
			"error": "Пользователь не найден",
		})
	case errors.Is(err, usecase.ErrUserForbidden):
		return c.JSON(http.StatusForbidden, echo.Map{
			"error": "Для этой операции требуются права администратора",
		})
	case errors.Is(err, usecase.ErrRoleDoesNotExist):
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Указана несуществующая роль",
		})
	case err != nil:
		c.Logger().Errorf("Ошибка при обновлении ролей пользователей в команде: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	return c.NoContent(http.StatusOK)
}

func (t *Team) Kick(c echo.Context) error {
	userID, err := t.authManager.CheckAuthFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Unauthorized",
		})
	}

	var request entity.KickUserRequest
	err = utils.ReadJSON(c, &request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}

	request.RequestUserID = userID

	err = t.teamUseCase.Kick(&request)
	switch {
	case errors.Is(err, usecase.ErrTeamNotFound):
		return c.JSON(http.StatusNotFound, echo.Map{
			"error": "Команда не найдена",
		})
	case errors.Is(err, usecase.ErrUserNotFound):
		return c.JSON(http.StatusNotFound, echo.Map{
			"error": "Пользователь не найден",
		})
	case errors.Is(err, usecase.ErrUserForbidden):
		return c.JSON(http.StatusForbidden, echo.Map{
			"error": "Для этой операции требуются права администратора",
		})
	case err != nil:
		c.Logger().Errorf("Ошибка при удалении пользователя из команды: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	return c.NoContent(http.StatusOK)
}

func (t *Team) SetVK(c echo.Context) error {
	userID, err := t.authManager.CheckAuthFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Unauthorized",
		})
	}

	var request entity.SetVKRequest
	err = utils.ReadJSON(c, &request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}

	request.RequestUserID = userID

	err = t.teamUseCase.SetVK(&request)
	switch {
	case errors.Is(err, usecase.ErrTeamNotFound):
		return c.JSON(http.StatusNotFound, echo.Map{
			"error": "Команда не найдена",
		})
	case errors.Is(err, usecase.ErrUserForbidden):
		return c.JSON(http.StatusForbidden, echo.Map{
			"error": "Для этой операции требуются права администратора",
		})
	case err != nil:
		c.Logger().Errorf("Ошибка при установке группы ВК: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	return c.NoContent(http.StatusOK)
}

func (t *Team) Platforms(c echo.Context) error {
	userID, err := t.authManager.CheckAuthFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Unauthorized",
		})
	}

	teamID, err := strconv.Atoi(c.QueryParam("team_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат team_id",
		})
	}

	platforms, err := t.teamUseCase.Platforms(userID, teamID)
	switch {
	case errors.Is(err, usecase.ErrTeamNotFound):
		return c.JSON(http.StatusNotFound, echo.Map{
			"error": "Команда не найдена",
		})
	case errors.Is(err, usecase.ErrUserForbidden):
		return c.JSON(http.StatusForbidden, echo.Map{
			"error": "Для этой операции требуются права администратора",
		})
	case err != nil:
		c.Logger().Errorf("Ошибка при получении платформ команды: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Произошла непредвиденная ошибка",
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"platforms": platforms,
	})
}
