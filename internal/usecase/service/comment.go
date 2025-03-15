package service

import (
	"errors"
	"fmt"
	"github.com/SevereCloud/vksdk/v3/api"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"net/http"
	"postic-backend/internal/entity"
	"strings"
)

type Comment struct {
	// commentRepo repo.Comment
	vkClient     *api.VK
	tgBot        *tgbotapi.BotAPI
	summarizeURL string // URL внешнего сервиса суммаризации
}

func NewComment(vkClient *api.VK, tgBot *tgbotapi.BotAPI, summarizeURL string) *Comment {
	return &Comment{
		vkClient:     vkClient,
		tgBot:        tgBot,
		summarizeURL: summarizeURL,
	}
}

func (c *Comment) Add(comment *entity.Comment) error {
	//TODO implement me
	panic("implement me")
}

func (c *Comment) Edit(comment *entity.Comment) error {
	//TODO implement me
	panic("implement me")
}

func (c *Comment) Delete(commentId int) error {
	//TODO implement me
	panic("implement me")
}

func (c *Comment) GetSlice(organizationID, offset, limit int) ([]entity.Comment, error) {
	//TODO implement me
	panic("implement me")
}

func (c *Comment) StartListener(organizationID, lastEvent int) (<-chan entity.Event, error) {
	//TODO implement me
	panic("implement me")
}

// SummarizeByPostURL получает пост по его url и суммаризирует комментарии под постом
// Вид url для vk: https://vk.com/wall-1_1
// Вид url для tg: https://t.me/c/1/1
func (c *Comment) SummarizeByPostURL(url string) (*entity.Summarize, error) {
	// Парсим URL
	if strings.Contains(url, "vk.com") {
		return c.summarizeVKPost(url)
	}
	if strings.Contains(url, "t.me") {
		return c.summarizeTGPost(url)
	}

	return nil, errors.New("unsupported URL format")
}

// summarizeVKPost обрабатывает пост и комментарии из VK
func (c *Comment) summarizeVKPost(url string) (*entity.Summarize, error) {
	// Извлекаем ownerID и postID из URL
	var ownerID, postID int
	_, err := fmt.Sscanf(url, "https://vk.com/wall%d_%d", &ownerID, &postID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse VK URL: %w", err)
	}

	// Получаем пост
	postResp, err := c.vkClient.WallGetByID(api.Params{
		"posts": fmt.Sprintf("%d_%d", ownerID, postID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get VK post: %w", err)
	}
	if len(postResp.Items) == 0 {
		return nil, errors.New("post not found")
	}
	// post := postResp.Items[0]

	// Получаем комментарии
	commentsResp, err := c.vkClient.WallGetComments(api.Params{
		"owner_id": ownerID,
		"post_id":  postID,
		"count":    100, // Максимальное количество комментариев
		"extended": 1,   // Получить информацию о пользователях
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get VK comments: %w", err)
	}

	// Собираем текст комментариев
	var commentsText []string
	for _, comment := range commentsResp.Items {
		commentsText = append(commentsText, comment.Text)
	}

	// Суммаризируем комментарии
	summary, err := c.summarizeText(commentsText)
	if err != nil {
		return nil, fmt.Errorf("failed to summarize comments: %w", err)
	}

	return &entity.Summarize{
		Markdown: summary,
		PostURL:  url,
	}, nil
}

// summarizeTGPost обрабатывает пост и комментарии из Telegram
func (c *Comment) summarizeTGPost(url string) (*entity.Summarize, error) {
	// Извлекаем chatID и messageID из URL
	var chatID, messageID int64
	_, err := fmt.Sscanf(url, "https://t.me/c/%d/%d", &chatID, &messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Telegram URL: %w", err)
	}
	/*
		// Получаем пост
		msg, err := c.tgBot.Me
		if err != nil {
			return nil, fmt.Errorf("failed to get Telegram post: %w", err)
		}

		// Получаем комментарии из группы для обсуждений
		// Предполагаем, что группа для обсуждений связана с каналом
		discussionGroupID := chatID // Замените на реальный ID группы для обсуждений
		comments, err := c.getTelegramComments(discussionGroupID, messageID)
		if err != nil {
			return nil, fmt.Errorf("failed to get Telegram comments: %w", err)
		}

		// Собираем текст комментариев
		var commentsText []string
		for _, comment := range comments {
			commentsText = append(commentsText, comment.Text)
		}

		// Суммаризируем текст поста и комментариев
		summary, err := c.summarizeText(msg.Text + "\n" + strings.Join(commentsText, "\n"))
		if err != nil {
			return nil, fmt.Errorf("failed to summarize post and comments: %w", err)
		}

	*/

	return &entity.Summarize{
		Markdown: "Суммарайз для комментариев Telegram временно не реализован",
		PostURL:  url,
	}, nil
}

// getTelegramComments получает комментарии из группы для обсуждений
func (c *Comment) getTelegramComments(chatID int64, messageID int) ([]tgbotapi.Message, error) {
	// Получаем все сообщения из группы для обсуждений
	updates, err := c.tgBot.GetUpdates(tgbotapi.UpdateConfig{
		Offset:  0,
		Limit:   100,
		Timeout: 60,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get Telegram updates: %w", err)
	}

	// Фильтруем сообщения, относящиеся к указанному посту
	var comments []tgbotapi.Message
	for _, update := range updates {
		if update.Message != nil && update.Message.ReplyToMessage != nil {
			if update.Message.ReplyToMessage.MessageID == messageID {
				comments = append(comments, *update.Message)
			}
		}
	}

	return comments, nil
}

// summarizeText отправляет текст на внешний сервис суммаризации
func (c *Comment) summarizeText(text []string) (string, error) {
	// Отправляем json с массивом комментариев
	resp, err := http.Post(
		c.summarizeURL,
		"application/json",
		strings.NewReader(fmt.Sprintf(`{"comments": %q}`, text)))
	if err != nil {
		return "", fmt.Errorf("failed to send request to summarize service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("summarize service returned status: %s", resp.Status)
	}

	var summary string
	_, err = fmt.Fscanf(resp.Body, "%s", &summary)
	if err != nil {
		return "", fmt.Errorf("failed to read summary from response: %w", err)
	}

	return summary, nil
}
