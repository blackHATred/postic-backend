package service

import (
	"bytes"
	"encoding/json"
	"github.com/labstack/gommon/log"
	"net/http"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"slices"
	"sync"
)

// subscriber идентифицирует подписчика на комментарии
type subscriber struct {
	userID      int
	teamID      int
	postUnionID int
}

type Comment struct {
	commentRepo      repo.Comment
	teamRepo         repo.Team
	telegramListener usecase.Listener
	telegramAction   usecase.CommentActionPlatform
	mlURL            string
	subscribers      map[subscriber]chan int
	mu               sync.Mutex
}

func NewComment(
	commentRepo repo.Comment,
	teamRepo repo.Team,
	telegramListener usecase.Listener,
	telegramAction usecase.CommentActionPlatform,
	mlURL string,
) usecase.Comment {
	return &Comment{
		commentRepo:      commentRepo,
		teamRepo:         teamRepo,
		telegramListener: telegramListener,
		telegramAction:   telegramAction,
		mlURL:            mlURL,
		subscribers:      make(map[subscriber]chan int),
	}
}

func (c *Comment) GetLastComments(request *entity.GetLastCommentsRequest) ([]*entity.Comment, error) {
	if request.Limit > 100 {
		request.Limit = 100
	}
	// Получаем комментарии из репозитория, используя текущее время как верхнюю границу
	// для получения самых последних комментариев
	comments, err := c.commentRepo.GetComments(request.PostUnionID, request.Offset, request.Limit)
	if err != nil {
		return nil, err
	}

	return comments, nil
}

func (c *Comment) GetSummarize(request *entity.SummarizeCommentRequest) (*entity.Summarize, error) {
	// проверяем права пользователя
	roles, err := c.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(roles, repo.AdminRole) && !slices.Contains(roles, repo.CommentsRole) {
		return nil, usecase.ErrUserForbidden
	}

	comments, err := c.commentRepo.GetLastComments(request.PostUnionID, 100)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Comments []string `json:"comments"`
	}
	for _, comment := range comments {
		if comment.Text == "" {
			continue
		}
		payload.Comments = append(payload.Comments, comment.Text)
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", c.mlURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	type ServerAnswer struct {
		Response string `json:"response"`
	}

	var serverAnswer ServerAnswer
	err = json.NewDecoder(resp.Body).Decode(&serverAnswer)
	if err != nil {
		return nil, err
	}
	log.Infof("response: %v", serverAnswer)
	return &entity.Summarize{
		Markdown:    serverAnswer.Response,
		PostUnionID: request.PostUnionID,
	}, nil
}

func (c *Comment) Subscribe(request *entity.SubscribeRequest) (<-chan int, error) {
	// проверяем права пользователя
	roles, err := c.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(roles, repo.AdminRole) && !slices.Contains(roles, repo.CommentsRole) {
		return nil, usecase.ErrUserForbidden
	}

	sub := subscriber{
		userID:      request.UserID,
		teamID:      request.TeamID,
		postUnionID: request.PostUnionID,
	}

	ch := make(chan int)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Если уже есть подписка, закрываем старый канал перед созданием нового
	if oldCh, exists := c.subscribers[sub]; exists {
		close(oldCh)
	}

	c.subscribers[sub] = ch

	go func() {
		tgCh := c.telegramListener.SubscribeToCommentEvents(request.TeamID, request.PostUnionID)
		// Пересылаем комментарии из Telegram в наш канал
		for commentID := range tgCh {
			log.Infof("commentID: %d", commentID)
			ch <- commentID
		}
	}()
	// в будущем нужно добавить другие платформы помимо телеграмма

	return ch, nil
}

func (c *Comment) Unsubscribe(request *entity.SubscribeRequest) {
	sub := subscriber{
		userID:      request.UserID,
		teamID:      request.TeamID,
		postUnionID: request.PostUnionID,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if ch, exists := c.subscribers[sub]; exists {
		close(ch)
		delete(c.subscribers, sub)

		c.telegramListener.UnsubscribeFromComments(request.TeamID, request.PostUnionID)
	}
}

func (c *Comment) ReplyComment(request *entity.ReplyCommentRequest) (int, error) {
	// проверяем права пользователя
	roles, err := c.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return 0, err
	}
	if !slices.Contains(roles, repo.AdminRole) && !slices.Contains(roles, repo.CommentsRole) {
		return 0, usecase.ErrUserForbidden
	}
	// получаем оригинальный комментарий
	comment, err := c.commentRepo.GetCommentInfo(request.CommentID)
	if err != nil {
		return 0, err
	}
	// делегируем отправку комментария в Telegram
	if comment.Platform == "tg" {
		return c.telegramAction.ReplyComment(request)
	}
	return 0, nil
}

func (c *Comment) DeleteComment(request *entity.DeleteCommentRequest) error {
	// проверяем права пользователя
	roles, err := c.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return err
	}
	if !slices.Contains(roles, repo.AdminRole) && !slices.Contains(roles, repo.CommentsRole) {
		return usecase.ErrUserForbidden
	}
	// получаем оригинальный комментарий
	comment, err := c.commentRepo.GetCommentInfo(request.PostCommentID)
	if err != nil {
		return err
	}
	// делегируем удаление комментария в Telegram
	if comment.Platform == "tg" {
		return c.telegramAction.DeleteComment(request)
	}
	return nil
}
