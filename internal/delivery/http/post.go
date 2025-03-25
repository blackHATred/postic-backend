package http

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"postic-backend/internal/delivery/http/utils"
	"postic-backend/internal/entity"
	"postic-backend/internal/usecase"
	"strconv"
)

type Post struct {
	cookiesManager *utils.CookieManager
	postUseCase    usecase.Post
}

func NewPost(cookiesManager *utils.CookieManager, postUseCase usecase.Post) *Post {
	return &Post{
		cookiesManager: cookiesManager,
		postUseCase:    postUseCase,
	}
}

func (p *Post) Configure(server *echo.Group) {
	server.POST("/add", p.AddPost)
	server.GET("/list", p.GetPosts)
	server.GET("/status/:id", p.GetPostStatus)
}

func (p *Post) AddPost(c echo.Context) error {
	userID, err := p.cookiesManager.GetUserIDFromContext(c)
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
	request.UserId = userID

	err = p.postUseCase.AddPost(request)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"status": "ok",
	})
}

func (p *Post) GetPosts(c echo.Context) error {
	userID, err := p.cookiesManager.GetUserIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}

	posts, err := p.postUseCase.GetPosts(userID)
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
	postID := c.Param("id")
	id, err := strconv.Atoi(postID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный ID поста",
		})
	}
	platform := c.QueryParam("platform")
	if platform == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Не указана платформа (query-param platform)",
		})
	}
	status, err := p.postUseCase.GetPostStatus(id, platform)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, echo.Map{
		"status": status,
	})
}
