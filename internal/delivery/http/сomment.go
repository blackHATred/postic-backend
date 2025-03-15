package http

import (
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"net/http"
	"postic-backend/internal/delivery/platform"
	"postic-backend/internal/entity"
	"postic-backend/internal/usecase"
	"sync"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Comment struct {
	commentUC usecase.Comment
}

func NewComment() *Comment {
	return &Comment{}
}

func (e *Comment) Configure(server *echo.Group, tg *platform.Tg, vk *platform.Vk) {
	wsHandler := NewWebSocketHandler(tg, vk)
	server.GET("/ws", wsHandler.HandleConnections)
}

type ClientSession struct {
	conn   *websocket.Conn
	events <-chan entity.Message
}

type WebSocketHandler struct {
	clients   map[*websocket.Conn]*ClientSession
	broadcast chan entity.Message
	mu        sync.Mutex
	tg        *platform.Tg
	vk        *platform.Vk
}

func NewWebSocketHandler(tg *platform.Tg, vk *platform.Vk) *WebSocketHandler {
	return &WebSocketHandler{
		clients:   make(map[*websocket.Conn]*ClientSession),
		broadcast: make(chan entity.Message),
		tg:        tg,
		vk:        vk,
	}
}

func (h *WebSocketHandler) HandleConnections(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	var initialMsg entity.ClientMessage
	err = ws.ReadJSON(&initialMsg)
	if err != nil {
		_ = ws.WriteJSON(echo.Map{"error": "неверный формат сообщения: " + err.Error()})
		return nil
	}
	if initialMsg.VkKey == "" && initialMsg.TgChatId == 0 {
		_ = ws.WriteJSON(echo.Map{"error": "нужен хотя бы один ключ"})
		return nil // Соединение уже захвачено, возвращаем nil
	}
	eventsChan := make(chan entity.Message)
	var tgEventsChan <-chan entity.Message
	var vkEventsChan <-chan entity.Message

	// добавляем чат в tg
	if initialMsg.TgChatId != 0 {
		tgEventsChan = h.tg.AddChat(initialMsg.TgChatId)
	}
	// добавляем группу в vk
	if initialMsg.VkKey != "" && initialMsg.VkGroupId != 0 {
		vkEventsChan, err = h.vk.AddGroup(initialMsg.VkKey, initialMsg.VkGroupId)
	}
	if err != nil {
		_ = ws.WriteJSON(echo.Map{"error": "не удалось добавить группу в vk: " + err.Error()})
		return nil
	}

	// объединяем каналы vk и tg
	go func() {
		for {
			select {
			case msg := <-tgEventsChan:
				eventsChan <- msg
			case msg := <-vkEventsChan:
				eventsChan <- msg
			}
		}
	}()

	session := &ClientSession{
		conn:   ws,
		events: eventsChan,
	}

	h.mu.Lock()
	h.clients[ws] = session
	h.mu.Unlock()

	// Отправляем все получаемые события клиенту
	for {
		select {
		case msg := <-eventsChan:
			err := ws.WriteJSON(msg)
			if err != nil {
				h.mu.Lock()
				delete(h.clients, ws)
				h.mu.Unlock()
				ws.Close() // Закрываем соединение явно
				return nil // Возвращаем nil, чтобы Echo не пытался управлять соединением
			}
		}
	}
}
