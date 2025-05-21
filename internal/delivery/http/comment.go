package http

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"net/http"
	"postic-backend/internal/delivery/http/utils"
	"postic-backend/internal/entity"
	"postic-backend/internal/usecase"
	"postic-backend/pkg/sse"
	"time"
)

type Comment struct {
	ctx            context.Context
	commentUseCase usecase.Comment
	authManager    utils.Auth
}

func NewComment(ctx context.Context, commentUseCase usecase.Comment, authManager utils.Auth) *Comment {
	return &Comment{
		ctx:            ctx,
		commentUseCase: commentUseCase,
		authManager:    authManager,
	}
}

func (c *Comment) Configure(server *echo.Group) {
	server.POST("/reply", c.ReplyToComment)
	server.DELETE("/delete", c.DeleteComment)
	server.GET("/summarize", c.Summarize)
	server.GET("/last", c.GetLastComments)
	server.GET("/get", c.GetComment)
	server.GET("/subscribe", c.SubscribeToComments)
	server.GET("/ideas", c.ReplyIdeas)
	server.POST("/mark", c.MarkAsTicket)
}

func (c *Comment) ReplyIdeas(e echo.Context) error {
	userID, err := c.authManager.CheckAuthFromContext(e)
	if err != nil {
		return e.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}

	request := &entity.ReplyIdeasRequest{}
	err = utils.ReadQuery(e, request)
	if err != nil {
		return e.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID

	replyIdeas, err := c.commentUseCase.ReplyIdeas(request)
	switch {
	case errors.Is(err, usecase.ErrCannotGenerateReplyIdeas):
		return e.JSON(http.StatusBadRequest, echo.Map{
			"error": "Не удалось сгенерировать идеи для ответа",
		})
	case errors.Is(err, usecase.ErrUserForbidden):
		return e.JSON(http.StatusForbidden, echo.Map{
			"error": "У вас нет прав на получение идей для ответа",
		})
	case err != nil:
		e.Logger().Error(err)
		return e.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Ошибка сервера",
		})
	}
	return e.JSON(http.StatusOK, replyIdeas)
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
		e.Logger().Error(err)
		return e.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Ошибка сервера",
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
		e.Logger().Error(err)
		return e.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Ошибка сервера",
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
		e.Logger().Error(err)
		return e.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Ошибка сервера",
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

	request := &entity.GetCommentsRequest{}
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
		e.Logger().Error(err)
		return e.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Ошибка сервера",
		})
	}
	return e.JSON(http.StatusOK, echo.Map{
		"status":   "ok",
		"comments": comments,
	})
}

func (c *Comment) GetComment(e echo.Context) error {
	userID, err := c.authManager.CheckAuthFromContext(e)
	if err != nil {
		return e.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}

	request := &entity.GetCommentRequest{}
	err = utils.ReadQuery(e, request)
	if err != nil {
		return e.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID

	comment, err := c.commentUseCase.GetComment(request)
	switch {
	case errors.Is(err, usecase.ErrUserForbidden):
		return e.JSON(http.StatusForbidden, echo.Map{
			"error": "У вас нет прав на получение комментария",
		})
	case err != nil:
		e.Logger().Error(err)
		return e.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Ошибка сервера",
		})
	}
	return e.JSON(http.StatusOK, echo.Map{
		"status":  "ok",
		"comment": comment,
	})
}

func (c *Comment) MarkAsTicket(e echo.Context) error {
	userID, err := c.authManager.CheckAuthFromContext(e)
	if err != nil {
		return e.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}

	request := &entity.MarkAsTicketRequest{}
	err = utils.ReadJSON(e, request)
	if err != nil {
		return e.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID

	err = c.commentUseCase.MarkAsTicket(request)
	switch {
	case errors.Is(err, usecase.ErrUserForbidden):
		return e.JSON(http.StatusForbidden, echo.Map{
			"error": "У вас нет прав на пометку комментария как тикет",
		})
	case errors.Is(err, usecase.ErrCommentNotFound):
		return e.JSON(http.StatusNotFound, echo.Map{
			"error": "Комментарий не найден",
		})
	case err != nil:
		e.Logger().Error(err)
		return e.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Ошибка сервера",
		})
	}
	return e.JSON(http.StatusOK, echo.Map{
		"status": "ok",
	})
}

func (c *Comment) SubscribeToComments(e echo.Context) error {
	userID, err := c.authManager.CheckAuthFromContext(e)
	if err != nil {
		return e.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}

	request := &entity.Subscriber{}
	err = utils.ReadQuery(e, request)
	if err != nil {
		return e.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат запроса",
		})
	}
	request.UserID = userID

	commentsCh, err := c.commentUseCase.Subscribe(e.Request().Context(), request)
	switch {
	case errors.Is(err, usecase.ErrUserForbidden):
		return echo.NewHTTPError(http.StatusForbidden, "У вас нет прав на получение комментариев")
	case errors.Is(err, usecase.ErrPostUnionNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "Пост не найден")
	case err != nil:
		log.Errorf("Ошибка при подписке на комментарии: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка сервера")
	}

	// Настраиваем SSE соединение
	w := e.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	w.Flush()

	pingTicker := time.NewTicker(20 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return nil
		case <-e.Request().Context().Done():
			return nil
		case comment, ok := <-commentsCh:
			if !ok {
				return nil
			}
			marshaledComment, err := json.Marshal(comment)
			if err != nil {
				log.Errorf("Ошибка при сериализации комментария: %v", err)
				return err
			}

			// Отправляем ID нового комментария клиенту
			event := sse.Event{
				Event: []byte("comment"),
				Data:  marshaledComment,
			}
			if err := event.MarshalTo(w); err != nil {
				log.Errorf("Ошибка при отправке комментария: %v", err)
				return err
			}
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
		case <-pingTicker.C:
			ping := sse.Event{
				Event: []byte("ping"),
				Data:  []byte(""),
			}
			if err := ping.MarshalTo(w); err != nil {
				log.Errorf("Ошибка маршалинга пинга: %v", err)
				return nil
			}
			w.Flush()
		}
	}
}
