package vkontakte

import (
	"fmt"
	"github.com/SevereCloud/vksdk/v3/api"
	"github.com/labstack/gommon/log"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"slices"
	"sync"
	"time"
)

type Comment struct {
	commentRepo repo.Comment
	teamRepo    repo.Team
	uploadRepo  repo.Upload
	subscribers map[entity.Subscriber]chan *entity.CommentEvent
	mu          sync.Mutex
}

func NewVkontakteComment(
	commentRepo repo.Comment,
	teamRepo repo.Team,
	uploadRepo repo.Upload,
) usecase.CommentActionPlatform {
	return &Comment{
		commentRepo: commentRepo,
		teamRepo:    teamRepo,
		uploadRepo:  uploadRepo,
		subscribers: make(map[entity.Subscriber]chan *entity.CommentEvent),
	}
}

func (c *Comment) SubscribeToCommentEvents(userID, teamID, postUnionID int) <-chan *entity.CommentEvent {
	// Create subscriber entity
	sub := entity.Subscriber{
		UserID:      userID,
		TeamID:      teamID,
		PostUnionID: postUnionID,
	}

	// Lock access to subscribers map for thread safety
	c.mu.Lock()
	defer c.mu.Unlock()

	// If subscriber already exists, return existing channel
	if ch, ok := c.subscribers[sub]; ok {
		return ch
	}

	// Create new channel for subscriber
	ch := make(chan *entity.CommentEvent)
	c.subscribers[sub] = ch
	return ch
}

func (c *Comment) UnsubscribeFromComments(userID, teamID, postUnionID int) {
	sub := entity.Subscriber{
		UserID:      userID,
		TeamID:      teamID,
		PostUnionID: postUnionID,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if ch, ok := c.subscribers[sub]; ok {
		close(ch)
		delete(c.subscribers, sub)
	}
}

func (c *Comment) ReplyComment(request *entity.ReplyCommentRequest) (int, error) {
	// Проверка прав пользов��теля
	roles, err := c.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return 0, err
	}
	if !slices.Contains(roles, repo.AdminRole) && !slices.Contains(roles, repo.CommentsRole) {
		return 0, usecase.ErrUserForbidden
	}

	// Оригинальный комментарий
	originalComment, err := c.commentRepo.GetCommentInfo(request.CommentID)
	if err != nil {
		return 0, err
	}

	// Креды ВК
	groupID, adminAPIKey, _, err := c.teamRepo.GetVKCredsByTeamID(request.TeamID)
	if err != nil {
		return 0, err
	}
	vk := api.NewVK(adminAPIKey)

	// Подготовка строки вложений (только для фото)
	attachmentsStr := ""
	for _, attachID := range request.Attachments {
		upload, err := c.uploadRepo.GetUpload(attachID)
		if err != nil {
			log.Errorf("Failed to get attachment: %v", err)
			continue
		}

		// Обрабатываем только фото
		if upload.FileType != "photo" {
			log.Warnf("Ignoring unsupported attachment type: %s", upload.FileType)
			continue
		}

		// Получаем сервер для загрузки фото
		photoParams := api.Params{
			"group_id": groupID,
		}
		server, err := vk.PhotosGetWallUploadServer(photoParams)
		if err != nil {
			return 0, fmt.Errorf("failed to get wall upload server: %w", err)
		}

		// Загружаем файл на сервер
		resp, err := vk.UploadFile(server.UploadURL, "phupload.RawBytes)
		if err != nil {
			return 0, fmt.Errorf("failed to upload photo: %w", err)
		}

		// Сохраняем загруженное фото
		saveParams := api.Params{
			"group_id": groupID,
			"server":   resp["server"],
			"photo":    resp["photo"],
			"hash":     resp["hash"],
		}
		photos, err := vk.PhotosSaveWallPhoto(saveParams)
		if err != nil {
			return 0, fmt.Errorf("failed to save wall photo: %w", err)
		}

		if len(photos) > 0 {
			if attachmentsStr != "" {
				attachmentsStr += ","
			}
			attachmentsStr += fmt.Sprintf("photo%d_%d", photos[0].OwnerID, photos[0].ID)
		}
	}

	// Публикация ответа на комментарий
	params := api.Params{
		"owner_id":         -groupID, // Отрицательный ID для группы
		"post_id":          originalComment.PostPlatformID,
		"message":          request.Text,
		"reply_to_comment": originalComment.CommentPlatformID,
		"from_group":       1, // От имени группы
	}

	if attachmentsStr != "" {
		params["attachments"] = attachmentsStr
	}

	response, err := vk.WallCreateComment(params)
	if err != nil {
		return 0, fmt.Errorf("failed to create comment: %w", err)
	}

	// Создаём комментарий
	comment := &entity.Comment{
		TeamID:            request.TeamID,
		PostUnionID:       originalComment.PostUnionID,
		Platform:          "vk",
		PostPlatformID:    originalComment.PostPlatformID,
		UserPlatformID:    0, // 0 означает комментарий от имени команды
		CommentPlatformID: response.CommentID,
		FullName:          "Ответ от имени команды",
		Username:          "",
		Text:              request.Text,
		IsTeamReply:       true,
		ReplyToCommentID:  request.CommentID,
		CreatedAt:         time.Now(),
		Attachments:       make([]*entity.Upload, 0),
	}

	// Сохраняем комментарий в БД
	commentID, err := c.commentRepo.AddComment(comment)
	if err != nil {
		log.Errorf("Failed to save team reply comment: %v", err)
		return 0, err
	}

	return commentID, nil
}

func (c *Comment) DeleteComment(request *entity.DeleteCommentRequest) error {
	// Get comment information
	comment, err := c.commentRepo.GetCommentInfo(request.PostCommentID)
	if err != nil {
		return err
	}

	// Get VK credentials
	groupID, adminAPIKey, _, err := c.teamRepo.GetVKCredsByTeamID(request.TeamID)
	if err != nil {
		return err
	}

	// Initialize VK API client
	vk := api.NewVK(adminAPIKey)

	// Delete comment via VK API
	params := api.Params{
		"owner_id":   -groupID, // Negative sign for community ID
		"comment_id": comment.CommentPlatformID,
	}

	_, err = vk.WallDeleteComment(params)
	if err != nil {
		return fmt.Errorf("failed to delete VK comment: %w", err)
	}

	// Delete comment from database
	err = c.commentRepo.DeleteComment(request.PostCommentID)
	if err != nil {
		log.Errorf("Failed to delete comment from database: %v", err)
		return err
	}

	// Notify subscribers about deleted comment
	postUnionId := 0
	if comment.PostUnionID != nil {
		postUnionId = *comment.PostUnionID
	}
	err = c.notifySubscribers(request.PostCommentID, postUnionId, request.TeamID, "deleted")
	if err != nil {
		log.Errorf("Failed to notify subscribers about deleted comment: %v", err)
	}

	return nil
}

// notifySubscribers sends notifications to subscribers about comment events
func (c *Comment) notifySubscribers(commentID, postUnionID, teamID int, eventType string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Get team members
	teamMemberIDs, err := c.teamRepo.GetTeamUsers(teamID)
	if err != nil {
		log.Errorf("Failed to get team members: %v", err)
		return err
	}

	for _, memberID := range teamMemberIDs {
		// Notify subscribers for team-level events
		sub := entity.Subscriber{
			UserID:      memberID,
			TeamID:      teamID,
			PostUnionID: 0,
		}
		if ch, ok := c.subscribers[sub]; ok {
			go func() {
				ch <- &entity.CommentEvent{
					CommentID: commentID,
					Type:      eventType,
				}
			}()
		}

		// Notify subscribers for post-level events
		if postUnionID != 0 {
			sub.PostUnionID = postUnionID
			if ch, ok := c.subscribers[sub]; ok {
				go func() {
					ch <- &entity.CommentEvent{
						CommentID: commentID,
						Type:      eventType,
					}
				}()
			}
		}
	}
	return nil
}
