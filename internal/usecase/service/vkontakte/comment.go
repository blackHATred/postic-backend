package vkontakte

import (
	"fmt"
	"github.com/SevereCloud/vksdk/v3/api"
	"github.com/labstack/gommon/log"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"strings"
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
	sub := entity.Subscriber{
		UserID:      userID,
		TeamID:      teamID,
		PostUnionID: postUnionID,
	}
	c.mu.Lock()
	if ch, ok := c.subscribers[sub]; ok {
		c.mu.Unlock()
		return ch
	}
	ch := make(chan *entity.CommentEvent)
	c.subscribers[sub] = ch
	c.mu.Unlock()
	return ch
}

func (c *Comment) UnsubscribeFromComments(userID, teamID, postUnionID int) {
	sub := entity.Subscriber{
		UserID:      userID,
		TeamID:      teamID,
		PostUnionID: postUnionID,
	}
	c.mu.Lock()
	if ch, ok := c.subscribers[sub]; ok {
		close(ch)
		delete(c.subscribers, sub)
	}
	c.mu.Unlock()
}

func (c *Comment) ReplyComment(request *entity.ReplyCommentRequest) (int, error) {
	// Получаем информацию о комментарии
	comment, err := c.commentRepo.GetCommentInfo(request.CommentID)
	if err != nil {
		return 0, err
	}
	if comment.PostPlatformID == nil {
		return 0, fmt.Errorf("comment not found")
	}

	// Получаем данные группы и ключ администратора
	groupID, adminAPIKey, _, err := c.teamRepo.GetVKCredsByTeamID(request.TeamID)
	if err != nil {
		return 0, err
	}

	vk := api.NewVK(adminAPIKey)

	// Подготавливаем параметры для ответа
	params := api.Params{
		"owner_id":         -groupID,
		"from_group":       groupID,
		"post_id":          *comment.PostPlatformID,
		"reply_to_comment": comment.CommentPlatformID,
		"message":          request.Text,
	}

	// Если есть вложения, обрабатываем их
	attachments := make([]string, 0)
	for _, attachment := range request.Attachments {
		upload, err := c.uploadRepo.GetUpload(attachment)
		if err != nil {
			log.Errorf("Failed to get upload: %v", err)
			return 0, err
		}
		switch upload.FileType {
		case "photo":
			photos, err := vk.UploadWallPhoto(upload.RawBytes)
			if err != nil {
				log.Errorf("Failed to upload photo: %v", err)
				return 0, err
			}
			if len(photos) == 0 {
				log.Error("No photos returned from upload")
				return 0, fmt.Errorf("no photos returned from upload")
			}
			photo := photos[len(photos)-1]
			attachments = append(attachments, fmt.Sprintf("photo%d_%d", photo.OwnerID, photo.ID))
		case "video":
			video, err := vk.UploadVideo(api.Params{
				"name":        "Video Reply",
				"description": "Uploaded via API",
				"wallpost":    0,
				"group_id":    groupID,
			}, upload.RawBytes)
			if err != nil {
				log.Errorf("Failed to upload video: %v", err)
				return 0, err
			}
			attachments = append(attachments, fmt.Sprintf("video%d_%d", video.OwnerID, video.VideoID))
		default:
			log.Warnf("Unsupported attachment type: %s", upload.FileType)
		}
	}

	// Добавляем вложения в параметры, если они есть
	if len(attachments) > 0 {
		params["attachments"] = strings.Join(attachments, ",")
	}

	// Отправляем ответ на комментарий
	response, err := vk.WallCreateComment(params)
	if err != nil {
		return 0, fmt.Errorf("failed to reply to VK comment: %w", err)
	}

	// Сохраняем комментарий в базе данных
	commentID, err := c.commentRepo.AddComment(&entity.Comment{
		TeamID:            request.TeamID,
		PostUnionID:       comment.PostUnionID,
		Platform:          "vk",
		PostPlatformID:    comment.PostPlatformID,
		UserPlatformID:    0, // 0 означает, что комментарий от имени бота/группы
		CommentPlatformID: response.CommentID,
		FullName:          "Ответ от имени команды",
		Username:          "",
		Text:              request.Text,
		IsTeamReply:       true,
		ReplyToCommentID:  request.CommentID,
		CreatedAt:         time.Now(),
		Attachments:       make([]*entity.Upload, 0),
	})
	// добавляем аттачи
	for _, attachment := range request.Attachments {
		upload := &entity.Upload{
			ID: attachment,
		}
		comment.Attachments = append(comment.Attachments, upload)
	}
	if err != nil {
		log.Errorf("Failed to save comment to database: %v", err)
		return 0, err
	}

	// Уведомляем подписчиков
	postUnionId := 0
	if comment.PostUnionID != nil {
		postUnionId = *comment.PostUnionID
	}
	err = c.notifySubscribers(commentID, postUnionId, request.TeamID, "new")
	if err != nil {
		log.Errorf("Failed to notify subscribers about replied comment: %v", err)
	}

	return response.CommentID, nil
}
func (c *Comment) DeleteComment(request *entity.DeleteCommentRequest) error {
	comment, err := c.commentRepo.GetCommentInfo(request.PostCommentID)
	if err != nil {
		return err
	}

	groupID, adminAPIKey, _, err := c.teamRepo.GetVKCredsByTeamID(request.TeamID)
	if err != nil {
		return err
	}

	vk := api.NewVK(adminAPIKey)

	params := api.Params{
		"owner_id":   -groupID,
		"comment_id": comment.CommentPlatformID,
	}

	_, err = vk.WallDeleteComment(params)
	if err != nil {
		return fmt.Errorf("failed to delete VK comment: %w", err)
	}

	err = c.commentRepo.DeleteComment(request.PostCommentID)
	if err != nil {
		log.Errorf("Failed to delete comment from database: %v", err)
		return err
	}

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

func (c *Comment) notifySubscribers(commentID, postUnionID, teamID int, eventType string) error {
	teamMemberIDs, err := c.teamRepo.GetTeamUsers(teamID)
	if err != nil {
		log.Errorf("Failed to get team members: %v", err)
		return err
	}

	for _, memberID := range teamMemberIDs {
		sub := entity.Subscriber{
			UserID:      memberID,
			TeamID:      teamID,
			PostUnionID: 0,
		}
		c.mu.Lock()
		if ch, ok := c.subscribers[sub]; ok {
			go func() {
				ch <- &entity.CommentEvent{
					CommentID: commentID,
					Type:      eventType,
				}
			}()
		}
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
		c.mu.Unlock()
	}
	return nil
}
