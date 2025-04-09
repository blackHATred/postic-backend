package http

import (
	"errors"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"net/http"
	"postic-backend/internal/delivery/http/utils"
	"postic-backend/internal/entity"
	"postic-backend/internal/usecase"
	"postic-backend/pkg/sse"
	"strconv"
	"time"
)

type Comment struct {
	commentUseCase usecase.Comment
	authManager    utils.Auth
}

func NewComment(commentUseCase usecase.Comment, authManager utils.Auth) *Comment {
	return &Comment{
		commentUseCase: commentUseCase,
		authManager:    authManager,
	}
}

func (c *Comment) Configure(server *echo.Group) {
	server.POST("/reply", c.ReplyToComment)
	server.DELETE("/delete", c.DeleteComment)
	server.GET("/summarize", c.Summarize)
	server.GET("/last", c.GetLastComments)
	server.GET("/subscribe", c.SubscribeToComments)
}

func (c *Comment) ReplyToComment(e echo.Context) error {
	userID, err := c.authManager.CheckAuthFromContext(e)
	if err != nil {
		return e.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}

	request := &entity.ReplyCommentRequest{}
	err = utils.ReadJSON(e, request)
	if err != nil {
		return e.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID

	commentId, err := c.commentUseCase.ReplyComment(request)
	switch {
	case errors.Is(err, usecase.ErrReplyCommentUnavailable):
		return e.JSON(http.StatusBadRequest, echo.Map{
			"error": "Невозможно ответить на комментарий. Возможно, он был удален отправителем",
		})
	case errors.Is(err, usecase.ErrUserForbidden):
		return e.JSON(http.StatusForbidden, echo.Map{
			"error": "У вас нет прав на ответ на комментарий",
		})
	case err != nil:
		return e.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}
	return e.JSON(http.StatusOK, echo.Map{
		"status":     "ok",
		"comment_id": commentId,
	})
}

func (c *Comment) DeleteComment(e echo.Context) error {
	userID, err := c.authManager.CheckAuthFromContext(e)
	if err != nil {
		return e.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}

	request := &entity.DeleteCommentRequest{}
	err = utils.ReadJSON(e, request)
	if err != nil {
		return e.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID

	err = c.commentUseCase.DeleteComment(request)
	switch {
	case errors.Is(err, usecase.ErrReplyCommentUnavailable):
		return e.JSON(http.StatusBadRequest, echo.Map{
			"error": "Невозможно удалить комментарий. Возможно, он был удален отправителем",
		})
	case errors.Is(err, usecase.ErrUserForbidden):
		return e.JSON(http.StatusForbidden, echo.Map{
			"error": "У вас нет прав на удаление комментария",
		})
	case err != nil:
		return e.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}
	return e.JSON(http.StatusOK, echo.Map{
		"status": "ok",
	})
}

func (c *Comment) Summarize(e echo.Context) error {
	userID, err := c.authManager.CheckAuthFromContext(e)
	if err != nil {
		return e.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}

	request := &entity.SummarizeCommentRequest{}
	err = utils.ReadQuery(e, request)
	if err != nil {
		return e.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID

	summarize, err := c.commentUseCase.GetSummarize(request)
	switch {
	case errors.Is(err, usecase.ErrUserForbidden):
		return e.JSON(http.StatusForbidden, echo.Map{
			"error": "У вас нет прав на получение сводки",
		})
	case err != nil:
		return e.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}
	return e.JSON(http.StatusOK, echo.Map{
		"status":    "ok",
		"summarize": summarize,
	})
}

func (c *Comment) GetLastComments(e echo.Context) error {
	userID, err := c.authManager.CheckAuthFromContext(e)
	if err != nil {
		return e.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}

	request := &entity.GetLastCommentsRequest{}
	err = utils.ReadQuery(e, request)
	if err != nil {
		return e.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID

	comments, err := c.commentUseCase.GetLastComments(request)
	switch {
	case errors.Is(err, usecase.ErrUserForbidden):
		return e.JSON(http.StatusForbidden, echo.Map{
			"error": "У вас нет прав на получение комментариев",
		})
	case err != nil:
		return e.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}
	return e.JSON(http.StatusOK, echo.Map{
		"status":   "ok",
		"comments": comments,
	})
}

func (c *Comment) SubscribeToComments(e echo.Context) error {
	// Получаем ID пользователя из контекста
	userID, err := c.authManager.CheckAuthFromContext(e)
	if err != nil {
		return e.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}

	request := &entity.SubscribeRequest{}
	err = utils.ReadQuery(e, request)
	if err != nil {
		return e.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID

	// Подписываемся на комментарии (в сервисе проверяются права доступа)
	commentsCh, err := c.commentUseCase.Subscribe(request)
	if err != nil {
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	}

	// Настраиваем SSE соединение
	w := e.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	w.Flush()

	// При выходе из функции отписываемся от комментариев
	defer c.commentUseCase.Unsubscribe(request)

	// Отправка периодических пингов для поддержания соединения
	pingTicker := time.NewTicker(20 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-e.Request().Context().Done():
			log.Infof("SSE клиент отключился, пользователь: %d, IP: %v", userID, e.RealIP())
			return nil

		case commentID, ok := <-commentsCh:
			log.Infof("Получен новый комментарий: %d, пользователь: %d, IP: %v", commentID, userID, e.RealIP())
			if !ok {
				// Канал был закрыт сервером
				return nil
			}

			// Отправляем ID нового комментария клиенту
			event := sse.Event{
				Event: []byte("comment"),
				Data:  []byte(strconv.Itoa(commentID)),
			}
			if err := event.MarshalTo(w); err != nil {
				log.Errorf("Ошибка при отправке комментария: %v", err)
				return err
			}
			w.Flush()

		case <-pingTicker.C:
			// Отправляем ping для поддержания соединения
			ping := sse.Event{
				Event: []byte("ping"),
				Data:  []byte(""),
			}
			if err := ping.MarshalTo(w); err != nil {
				log.Errorf("Ошибка при отправке ping: %v", err)
				return err
			}
			w.Flush()
		}
	}
}
