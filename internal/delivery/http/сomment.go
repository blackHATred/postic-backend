package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SevereCloud/vksdk/v3/api"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"net/http"
	"postic-backend/internal/delivery/platform"
	"postic-backend/internal/entity"
	"postic-backend/internal/usecase"
	"postic-backend/internal/usecase/service"
	"strings"
	"sync"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return false
	},
}

type Comment struct {
	commentUC    usecase.Comment
	summarizeURL string
	tg           *service.Telegram
	vk           *platform.Vk
	vkApi        *api.VK
	vkMessages   map[int][]entity.Message // postId -> []message
	mu           sync.Mutex
}

func NewComment(commentUC usecase.Comment, tg *service.Telegram, vk *platform.Vk, summarizeURL string, vkApi *api.VK) *Comment {
	return &Comment{
		commentUC:    commentUC,
		summarizeURL: summarizeURL,
		vkMessages:   make(map[int][]entity.Message),
		tg:           tg,
		vk:           vk,
		vkApi:        vkApi,
	}
}

func (e *Comment) Configure(server *echo.Group) {
	wsHandler := NewWebSocketHandler(e)

	server.GET("/ws", wsHandler.HandleConnections)
	server.GET("/summary", e.Summary)
}

func (e *Comment) Summary(c echo.Context) error {
	// Получаем URL поста
	url := c.QueryParam("url")
	if url == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "url не указан"})
	}
	summary, err := e.summarizeVKPost(url)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, summary)
}

type ClientSession struct {
	conn   *websocket.Conn
	events <-chan entity.Message
}

type WebSocketHandler struct {
	commentDelivery *Comment
	clients         map[*websocket.Conn]*ClientSession
	broadcast       chan entity.Message
	mu              sync.Mutex
}

func NewWebSocketHandler(commentDelivery *Comment) *WebSocketHandler {
	return &WebSocketHandler{
		clients:         make(map[*websocket.Conn]*ClientSession),
		broadcast:       make(chan entity.Message),
		commentDelivery: commentDelivery,
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
		//tgEventsChan = h.commentDelivery.tg.AddChat(initialMsg.TgChatId)
	}
	// добавляем группу в vk
	if initialMsg.VkKey != "" && initialMsg.VkGroupId != 0 {
		vkEventsChan, err = h.commentDelivery.vk.AddGroup(initialMsg.VkKey, initialMsg.VkGroupId)
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
				// сообщения из вк сохраняем для суммарайзера
				h.commentDelivery.mu.Lock()
				switch msg.Type {
				case "new":
					h.commentDelivery.vkMessages[msg.PostId] = append(h.commentDelivery.vkMessages[msg.PostId], msg)
				case "update":
					if _, ok := h.commentDelivery.vkMessages[msg.PostId]; ok {
						for i, m := range h.commentDelivery.vkMessages[msg.PostId] {
							if m.Id == msg.Id {
								h.commentDelivery.vkMessages[msg.PostId][i] = msg
								break
							}
						}
					}
				case "delete":
					if _, ok := h.commentDelivery.vkMessages[msg.PostId]; ok {
						for i, m := range h.commentDelivery.vkMessages[msg.PostId] {
							if m.Id == msg.Id {
								h.commentDelivery.vkMessages[msg.PostId] = append(h.commentDelivery.vkMessages[msg.PostId][:i], h.commentDelivery.vkMessages[msg.PostId][i+1:]...)
								break
							}
						}
					}
				}
				h.commentDelivery.mu.Unlock()
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

// summarizeVKPost обрабатывает пост и комментарии из VK
func (e *Comment) summarizeVKPost(url string) (*entity.Summarize, error) {
	// Извлекаем ownerID и postID из URL
	var ownerID, postID int
	_, err := fmt.Sscanf(url, "https://vk.com/wall%d_%d", &ownerID, &postID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse VK URL: %w", err)
	}

	// смотрим сообщения
	e.mu.Lock()
	msgs, ok := e.vkMessages[postID]
	e.mu.Unlock()
	if !ok {
		return nil, errors.New("пост не найден или под ним нет новых комментариев")
	}

	commentsText := make([]string, 0)
	// Собираем текст комментариев
	for _, comment := range msgs {
		commentsText = append(commentsText, comment.Text)
	}

	// Суммаризируем комментарии
	summary, err := e.summarizeText(commentsText)
	if err != nil {
		return nil, fmt.Errorf("failed to summarize comments: %w", err)
	}

	return &entity.Summarize{
		Markdown: summary,
		PostURL:  url,
	}, nil
}

// summarizeText отправляет текст на внешний сервис суммаризации
func (e *Comment) summarizeText(text []string) (string, error) {
	// Отправляем json с массивом комментариев
	resp, err := http.Post(
		e.summarizeURL,
		"application/json",
		strings.NewReader(fmt.Sprintf(`{"comments": %q}`, text)))
	if err != nil {
		return "", fmt.Errorf("failed to send request to summarize service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("summarize service returned status: %s", resp.Status)
	}

	var result struct {
		Markdown string `json:"markdown"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", fmt.Errorf("failed to read summary from response: %w", err)
	}

	return result.Markdown, nil
}
