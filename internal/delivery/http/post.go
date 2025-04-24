package http

import (
	"errors"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"net/http"
	"postic-backend/internal/delivery/http/utils"
	"postic-backend/internal/entity"
	"postic-backend/internal/usecase"
	"time"
)

type Post struct {
	authManager utils.Auth
	postUseCase usecase.PostUnion
}

func NewPost(authManager utils.Auth, postUseCase usecase.PostUnion) *Post {
	return &Post{
		authManager: authManager,
		postUseCase: postUseCase,
	}
}

func (p *Post) Configure(server *echo.Group) {
	server.POST("/add", p.AddPost)
	server.POST("/edit", p.EditPost)
	server.DELETE("/delete", p.DeletePost)
	server.POST("/action", p.DoAction)
	server.GET("/get", p.GetPost)
	server.GET("/list", p.GetPosts)
	server.GET("/status", p.GetPostStatus)
}

func (p *Post) AddPost(c echo.Context) error {
	userID, err := p.authManager.CheckAuthFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}

	request := &entity.AddPostRequest{}
	err = utils.ReadJSON(c, request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID

	log.Infof("%v", request)

	postId, actionIDs, err := p.postUseCase.AddPostUnion(request)
	switch {
	case errors.Is(err, usecase.ErrUserForbidden):
		return c.JSON(http.StatusForbidden, echo.Map{
			"error": "У вас нет прав на создание постов в этой команде",
		})
	case err != nil:
		c.Logger().Errorf("error adding post: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"status":    "ok",
		"actionIDs": actionIDs,
		"post_id":   postId,
	})
}

func (p *Post) EditPost(c echo.Context) error {
	userID, err := p.authManager.CheckAuthFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}

	request := &entity.EditPostRequest{}
	err = utils.ReadJSON(c, request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID
	actionIDs, err := p.postUseCase.EditPostUnion(request)
	switch {
	case errors.Is(err, usecase.ErrUserForbidden):
		return c.JSON(http.StatusForbidden, echo.Map{
			"error": "У вас нет прав на редактирование постов в этой команде",
		})
	case err != nil:
		c.Logger().Errorf("error editing post: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"status":    "ok",
		"actionIDs": actionIDs,
	})
}

func (p *Post) DeletePost(c echo.Context) error {
	userID, err := p.authManager.CheckAuthFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}

	request := &entity.DeletePostRequest{}
	err = utils.ReadJSON(c, request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID
	actionIDs, err := p.postUseCase.DeletePostUnion(request)
	switch {
	case errors.Is(err, usecase.ErrUserForbidden):
		return c.JSON(http.StatusForbidden, echo.Map{
			"error": "У вас нет прав на удаление постов в этой команде",
		})
	case err != nil:
		c.Logger().Errorf("error deleting post: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"status":    "ok",
		"actionIDs": actionIDs,
	})
}

func (p *Post) DoAction(c echo.Context) error {
	userID, err := p.authManager.CheckAuthFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}

	request := &entity.DoActionRequest{}
	err = utils.ReadJSON(c, request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID
	actionID, err := p.postUseCase.DoAction(request)
	switch {
	case errors.Is(err, usecase.ErrUserForbidden):
		return c.JSON(http.StatusForbidden, echo.Map{
			"error": "У вас нет прав на выполнение действий в этой команде",
		})
	case err != nil:
		c.Logger().Errorf("error doing action: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"status":   "ok",
		"actionID": actionID,
	})
}

func (p *Post) GetPost(c echo.Context) error {
	userID, err := p.authManager.CheckAuthFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}

	request := &entity.GetPostRequest{}
	err = utils.ReadQuery(c, request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID
	post, err := p.postUseCase.GetPostUnion(request)
	switch {
	case errors.Is(err, usecase.ErrUserForbidden):
		return c.JSON(http.StatusForbidden, echo.Map{
			"error": "У вас нет прав на получение постов в этой команде",
		})
	case err != nil:
		c.Logger().Errorf("error getting post: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"post": post,
	})
}

func (p *Post) GetPosts(c echo.Context) error {
	userID, err := p.authManager.CheckAuthFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}

	request := &entity.GetPostsRequest{}
	err = utils.ReadQuery(c, request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID
	if request.Offset == nil {
		currentTime := time.Now()
		request.Offset = &currentTime
	}
	posts, err := p.postUseCase.GetPosts(request)
	switch {
	case errors.Is(err, usecase.ErrUserForbidden):
		return c.JSON(http.StatusForbidden, echo.Map{
			"error": "У вас нет прав на получение постов в этой команде",
		})
	case err != nil:
		c.Logger().Errorf("error getting posts: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"posts": posts,
	})
}

func (p *Post) GetPostStatus(c echo.Context) error {
	userID, err := p.authManager.CheckAuthFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}
	request := &entity.PostStatusRequest{}
	err = utils.ReadQuery(c, request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID
	status, err := p.postUseCase.GetPostStatus(request)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, echo.Map{
		"status": status,
	})
}
