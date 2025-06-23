package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"slices"
	"strings"

	"github.com/labstack/gommon/log"
)

type Comment struct {
	commentRepo     repo.Comment
	postRepo        repo.Post
	teamRepo        repo.Team
	telegramAction  usecase.CommentActionPlatform
	vkontakteAction usecase.CommentActionPlatform
	summarizeURL    string
	replyIdeasURL   string
	eventRepo       repo.CommentEventRepository // Kafka-репозиторий событий комментариев
}

func NewComment(
	commentRepo repo.Comment,
	postRepo repo.Post,
	teamRepo repo.Team,
	telegramAction usecase.CommentActionPlatform,
	vkontakteAction usecase.CommentActionPlatform,
	summarizeURL string,
	replyIdeasURL string,
	eventRepo repo.CommentEventRepository,
) usecase.Comment {
	return &Comment{
		commentRepo:     commentRepo,
		postRepo:        postRepo,
		teamRepo:        teamRepo,
		telegramAction:  telegramAction,
		vkontakteAction: vkontakteAction,
		summarizeURL:    summarizeURL,
		replyIdeasURL:   replyIdeasURL,
		eventRepo:       eventRepo,
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

	comment, err := c.commentRepo.GetComment(request.CommentID)
	switch {
	case errors.Is(err, repo.ErrCommentNotFound):
		// комментарий не найден, значит, его удалили
		return &entity.ReplyIdeasResponse{Ideas: []string{}}, nil
	case err != nil:
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
	req, err := http.NewRequest("POST", c.replyIdeasURL, bytes.NewBuffer(jsonData))
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
	//log.Infof("response: %v", serverAnswer)

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

	comment, err := c.commentRepo.GetComment(request.CommentID)
	switch {
	case errors.Is(err, repo.ErrCommentNotFound):
		// комментарий не найден, значит, его удалили
		return nil, usecase.ErrCommentNotFound
	case err != nil:
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

	if request.Limit == 0 || request.Limit > 100 {
		request.Limit = 100
	}

	// Получаем комментарии из репозитория, используя текущее время как верхнюю границу
	// для получения самых последних комментариев
	var comments []*entity.Comment
	if request.MarkedAsTicket == nil || !*request.MarkedAsTicket {
		comments, err = c.commentRepo.GetComments(request.TeamID, request.PostUnionID, request.Offset, request.Before, request.Limit)
	} else {
		comments, err = c.commentRepo.GetTicketComments(request.TeamID, request.Offset, request.Before, request.Limit)
	}
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
		Comments string `json:"comments"`
	}
	if len(comments) == 0 {
		payload.Comments = "Нет комментариев для суммаризации"
	} else {
		var builder strings.Builder
		for _, comment := range comments {
			builder.WriteString(comment.Text)
			builder.WriteString("\n\n")
		}
		payload.Comments = builder.String()
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", c.summarizeURL, bytes.NewBuffer(jsonData))
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

// Subscribe подписывается на получение новых комментариев через Kafka
func (c *Comment) Subscribe(ctx context.Context, request *entity.Subscriber) (<-chan *entity.CommentEvent, error) {
	// Проверка прав доступа пользователя
	roles, err := c.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}

	// Проверка, имеет ли пользователь права админа или доступ к комментариям
	if !slices.Contains(roles, repo.AdminRole) && !slices.Contains(roles, repo.CommentsRole) {
		return nil, usecase.ErrUserForbidden
	}

	// Если указан PostUnionID, проверяем, что он принадлежит этой команде
	if request.PostUnionID != 0 {
		postUnion, err := c.postRepo.GetPostUnion(request.PostUnionID)
		if err != nil {
			if errors.Is(err, repo.ErrPostUnionNotFound) {
				return nil, usecase.ErrPostUnionNotFound
			}
			return nil, err
		}
		if postUnion.TeamID != request.TeamID {
			return nil, usecase.ErrUserForbidden
		}
	}

	// Подписываемся на события комментариев через Kafka
	ch, err := c.eventRepo.SubscribeCommentEvents(
		ctx,
		request.TeamID,
		request.PostUnionID,
	)
	if err != nil {
		return nil, err
	}

	return ch, nil
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
	comment, err := c.commentRepo.GetComment(request.CommentID)
	switch {
	case errors.Is(err, repo.ErrCommentNotFound):
		return 0, usecase.ErrCommentNotFound
	case err != nil:
		return 0, err
	}
	if comment.TeamID != request.TeamID {
		return 0, usecase.ErrUserForbidden
	}

	// валидация длины текста для платформы
	if err := request.IsValid(comment.Platform); err != nil {
		return 0, err
	}

	// делегируем отправку комментария
	switch comment.Platform {
	case "vk":
		return c.vkontakteAction.ReplyComment(request)
	case "tg":
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
	comment, err := c.commentRepo.GetComment(request.PostCommentID)
	switch {
	case errors.Is(err, repo.ErrCommentNotFound):
		return nil
	case err != nil:
		return err
	}
	if comment.TeamID != request.TeamID {
		return usecase.ErrUserForbidden
	}
	switch comment.Platform {
	case "vk":
		return c.vkontakteAction.DeleteComment(request)
	case "tg":
		return c.telegramAction.DeleteComment(request)
	}
	return nil
}

func (c *Comment) MarkAsTicket(request *entity.MarkAsTicketRequest) error {
	// проверяем права пользователя
	roles, err := c.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return err
	}
	if !slices.Contains(roles, repo.AdminRole) && !slices.Contains(roles, repo.CommentsRole) {
		return usecase.ErrUserForbidden
	}
	// получаем оригинальный комментарий
	comment, err := c.commentRepo.GetComment(request.PostCommentID)
	switch {
	case errors.Is(err, repo.ErrCommentNotFound):
		return usecase.ErrCommentNotFound
	case err != nil:
		return err
	}
	if comment.TeamID != request.TeamID {
		return usecase.ErrUserForbidden
	}
	// обновляем комментарий, помечая (или наоборот, убирая) его как тикет
	comment.MarkedAsTicket = request.MarkedAsTicket
	err = c.commentRepo.EditComment(comment)
	if err != nil {
		return err
	}
	return nil
}
