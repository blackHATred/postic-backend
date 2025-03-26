package http

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"net/http"
	"postic-backend/internal/entity"
	"postic-backend/internal/usecase"
	"strconv"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Comment struct {
	tgUseCase usecase.Telegram
}

func NewComment(tgUseCase usecase.Telegram) *Comment {
	return &Comment{
		tgUseCase: tgUseCase,
	}
}

func (c *Comment) Configure(server *echo.Group) {
	server.GET("/ws", c.handleWSConnection)
	server.GET("/user/tg/:id", c.getTGUserInfo)
}

func (c *Comment) getTGUserInfo(ctx echo.Context) error {
	userID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "неверный формат id пользователя",
		})
	}
	user, err := c.tgUseCase.GetUser(userID)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}
	return ctx.JSON(http.StatusOK, user)
}

func (c *Comment) handleWSConnection(ctx echo.Context) error {
	ws, err := upgrader.Upgrade(ctx.Response(), ctx.Request(), nil)
	if err != nil {
		return err
	}
	defer func() { _ = ws.Close() }()
	newCommentsChan := c.tgUseCase.Subscribe()
	done := make(chan struct{})

	go func() {
		defer close(done)
		var parsedRequest *entity.WebSocketCommentRequest
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				ctx.Logger().Error("Read error:", err)
				return
			}
			log.Infof("Received from client: %s\n", msg)
			err = json.Unmarshal(msg, &parsedRequest)
			if err != nil {
				err := ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("невалидный формат запроса: %s", err.Error())))
				if err != nil {
					return
				}
				continue
			}
			if parsedRequest.Type == "get_comments" && parsedRequest.GetComments != nil {
				comments, err := c.tgUseCase.GetComments(parsedRequest.GetComments.PostUnionID, parsedRequest.GetComments.Offset, 10)
				if err != nil {
					err := ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("ошибка при получении комментариев: %s", err.Error())))
					if err != nil {
						return
					}
					continue
				}
				jsonBytes, err := json.Marshal(struct {
					Comments []*entity.TelegramComment `json:"comments"`
				}{
					Comments: comments,
				})
				if err != nil {
					err := ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("ошибка при маршалинге комментариев: %s", err.Error())))
					if err != nil {
						return
					}
					continue
				}
				err = ws.WriteMessage(websocket.TextMessage, jsonBytes)
				if err != nil {
					return
				}
			} else {
				err := ws.WriteMessage(websocket.TextMessage, []byte("неверный тип запроса"))
				if err != nil {
					return
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case comment := <-newCommentsChan:
				jsonBytes, err := json.Marshal(comment)
				if err != nil {
					ctx.Logger().Error("Marshal error:", err)
					return
				}
				err = ws.WriteMessage(websocket.TextMessage, jsonBytes)
				if err != nil {
					ctx.Logger().Error("Write error:", err)
					return
				}
			case <-done: // Если чтение закрылось, завершаем отправку
				return
			}
		}
	}()

	// Ждём завершения чтения
	<-done
	return nil
}
