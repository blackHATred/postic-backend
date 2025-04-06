package http

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"postic-backend/internal/delivery/http/utils"
	"postic-backend/internal/entity"
	"postic-backend/internal/usecase"
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

	postId, actionIDs, err := p.postUseCase.AddPostUnion(request)
	if err != nil {
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

func (p *Post) GetPosts(c echo.Context) error {
	userID, err := p.authManager.CheckAuthFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}

	request := &entity.GetPostsRequest{}
	err = utils.ReadJSON(c, request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID
	request.Limit = 10
	posts, err := p.postUseCase.GetPosts(request)
	if err != nil {
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
	err = utils.ReadJSON(c, request)
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
