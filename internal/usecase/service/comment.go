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

type Comment struct {
	commentRepo      repo.Comment
	postRepo         repo.Post
	teamRepo         repo.Team
	telegramListener usecase.Listener
	telegramAction   usecase.CommentActionPlatform
	summarizeURL     string
	replyIdeasURL    string
	subscribers      map[entity.Subscriber]chan *entity.CommentEvent
	mu               sync.Mutex
}

func NewComment(
	commentRepo repo.Comment,
	postRepo repo.Post,
	teamRepo repo.Team,
	telegramListener usecase.Listener,
	telegramAction usecase.CommentActionPlatform,
	summarizeURL string,
	replyIdeasURL string,
) usecase.Comment {
	return &Comment{
		commentRepo:      commentRepo,
		postRepo:         postRepo,
		teamRepo:         teamRepo,
		telegramListener: telegramListener,
		telegramAction:   telegramAction,
		summarizeURL:     summarizeURL,
		replyIdeasURL:    replyIdeasURL,
		subscribers:      make(map[entity.Subscriber]chan *entity.CommentEvent),
	}
}

func (c *Comment) ReplyIdeas(request *entity.ReplyIdeasRequest) (*entity.ReplyIdeasResponse, error) {
	// проверяем права пользователя
	roles, err := c.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(roles, repo.AdminRole) && !slices.Contains(roles, repo.CommentsRole) {
		return nil, usecase.ErrUserForbidden
	}

	comment, err := c.commentRepo.GetCommentInfo(request.CommentID)
	if err != nil {
		return nil, err
	}

	if len(comment.Text) < 2 {
		// нет смысла что-то предлагать
		return &entity.ReplyIdeasResponse{Ideas: []string{}}, nil
	}

	type MLRequest struct {
		Comment string `json:"comment"`
	}

	jsonData, err := json.Marshal(MLRequest{Comment: comment.Text})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", c.replyIdeasURL, bytes.NewBuffer(jsonData))
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
		Warnings      string `json:"warnings"`
		NoAnswer      bool   `json:"no_answer"`
		SupportNeeded bool   `json:"support_needed"`
		Answer0       string `json:"answer_0,omitempty"`
		Answer1       string `json:"answer_1,omitempty"`
		Answer2       string `json:"answer_2,omitempty"`
	}

	var serverAnswer ServerAnswer
	err = json.NewDecoder(resp.Body).Decode(&serverAnswer)
	if err != nil {
		return nil, err
	}
	log.Infof("response: %v", serverAnswer)

	if serverAnswer.NoAnswer {
		// нет ответа
		return &entity.ReplyIdeasResponse{Ideas: []string{}}, nil
	}
	return &entity.ReplyIdeasResponse{Ideas: []string{serverAnswer.Answer0, serverAnswer.Answer1, serverAnswer.Answer2}}, nil
}

func (c *Comment) GetComment(request *entity.GetCommentRequest) (*entity.Comment, error) {
	// проверяем права пользователя
	roles, err := c.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(roles, repo.AdminRole) && !slices.Contains(roles, repo.CommentsRole) {
		return nil, usecase.ErrUserForbidden
	}

	comment, err := c.commentRepo.GetCommentInfo(request.CommentID)
	if err != nil {
		return nil, err
	}
	// проверяем, что комментарий принадлежит этой команде
	if comment.TeamID != request.TeamID {
		return nil, usecase.ErrUserForbidden
	}

	return comment, nil
}

func (c *Comment) GetLastComments(request *entity.GetCommentsRequest) ([]*entity.Comment, error) {
	// проверяем права пользователя
	roles, err := c.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(roles, repo.AdminRole) && !slices.Contains(roles, repo.CommentsRole) {
		return nil, usecase.ErrUserForbidden
	}

	if request.PostUnionID != 0 {
		// проверяем, что postUnion принадлежит этой команде
		postUnion, err := c.postRepo.GetPostUnion(request.PostUnionID)
		if err != nil {
			return nil, err
		}
		if postUnion.TeamID != request.TeamID {
			return nil, usecase.ErrUserForbidden
		}
	}

	if request.Limit > 100 {
		request.Limit = 100
	}

	// Получаем комментарии из репозитория, используя текущее время как верхнюю границу
	// для получения самых последних комментариев
	comments, err := c.commentRepo.GetComments(request.PostUnionID, request.Offset, request.Before, request.Limit)
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
	req, err := http.NewRequest("GET", c.summarizeURL, bytes.NewBuffer(jsonData))
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

func (c *Comment) Subscribe(request *entity.Subscriber) (<-chan *entity.CommentEvent, error) {
	// проверяем права пользователя
	roles, err := c.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(roles, repo.AdminRole) && !slices.Contains(roles, repo.CommentsRole) {
		return nil, usecase.ErrUserForbidden
	}

	sub := entity.Subscriber{
		UserID:      request.UserID,
		TeamID:      request.TeamID,
		PostUnionID: request.PostUnionID,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Если уже есть подписка, возвращаем тот же канал
	if oldCh, exists := c.subscribers[sub]; exists {
		return oldCh, nil
	}

	ch := make(chan *entity.CommentEvent)
	c.subscribers[sub] = ch

	go func() {
		// Подписываемся сразу в нескольких местах. Listener возвращает новые комментарии и редактирования,
		// а Action возвращает удаления комментариев другими модераторами
		tgListenerCh := c.telegramListener.SubscribeToCommentEvents(sub.UserID, sub.TeamID, sub.PostUnionID)
		tgActionCh := c.telegramAction.SubscribeToCommentEvents(sub.UserID, sub.TeamID, sub.PostUnionID)

		// объединяем каналы
		for {
			select {
			case comment, ok := <-tgListenerCh:
				if !ok {
					tgListenerCh = nil
					if tgActionCh == nil {
						// оба каналы закрыты
						return
					}
					continue
				}
				ch <- comment

			case comment, ok := <-tgActionCh:
				if !ok {
					tgActionCh = nil
					if tgListenerCh == nil {
						// оба канала закрыты
						return
					}
					continue
				}
				ch <- comment
			}
		}
	}()

	return ch, nil
}

func (c *Comment) Unsubscribe(request *entity.Subscriber) {
	sub := entity.Subscriber{
		UserID:      request.UserID,
		TeamID:      request.TeamID,
		PostUnionID: request.PostUnionID,
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if ch, exists := c.subscribers[sub]; exists {
		c.telegramListener.UnsubscribeFromComments(sub.UserID, sub.TeamID, sub.PostUnionID)
		c.telegramAction.UnsubscribeFromComments(sub.UserID, sub.TeamID, sub.PostUnionID)
		close(ch)
		delete(c.subscribers, sub)
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
