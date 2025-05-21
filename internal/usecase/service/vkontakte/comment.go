package vkontakte

import (
	"context"
	"fmt"
	"github.com/SevereCloud/vksdk/v3/api"
	"github.com/labstack/gommon/log"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"strings"
	"time"
)

type Comment struct {
	commentRepo repo.Comment
	teamRepo    repo.Team
	uploadRepo  repo.Upload
	eventRepo   repo.CommentEventRepository
}

func NewVkontakteComment(
	commentRepo repo.Comment,
	teamRepo repo.Team,
	uploadRepo repo.Upload,
	eventRepo repo.CommentEventRepository,
) *Comment {
	return &Comment{
		commentRepo: commentRepo,
		teamRepo:    teamRepo,
		uploadRepo:  uploadRepo,
		eventRepo:   eventRepo,
	}
}

func (c *Comment) ReplyComment(request *entity.ReplyCommentRequest) (int, error) {
	// Получаем информацию о комментарии
	comment, err := c.commentRepo.GetComment(request.CommentID)
	if err != nil {
		return 0, err
	}
	if comment.PostPlatformID == nil {
		return 0, fmt.Errorf("comment not found")
	}

	// Получаем данные группы и ключ администратора
	vkChannel, err := c.teamRepo.GetVKCredsByTeamID(request.TeamID)
	if err != nil {
		return 0, err
	}

	vk := api.NewVK(vkChannel.AdminAPIKey)

	// Подготавливаем параметры для ответа
	params := api.Params{
		"owner_id":         -vkChannel.GroupID,
		"from_group":       vkChannel.GroupID,
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
				"group_id":    vkChannel.GroupID,
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

	// Публикуем событие о новом комментарии в Kafka
	newComment, _ := c.commentRepo.GetComment(commentID)
	if newComment != nil {
		event := &entity.CommentEvent{
			EventID:    fmt.Sprintf("vk-%d-%d", newComment.TeamID, commentID),
			TeamID:     newComment.TeamID,
			PostID:     derefInt(newComment.PostUnionID),
			Type:       entity.CommentCreated,
			CommentID:  newComment.ID,
			OccurredAt: newComment.CreatedAt,
		}
		if err := c.eventRepo.PublishCommentEvent(context.Background(), event); err != nil {
			log.Errorf("Не удалось опубликовать событие о новом комментарии в Kafka: %v", err)
		}
	}

	return response.CommentID, nil
}
func (c *Comment) DeleteComment(request *entity.DeleteCommentRequest) error {
	comment, err := c.commentRepo.GetComment(request.PostCommentID)
	if err != nil {
		return err
	}

	vkChannel, err := c.teamRepo.GetVKCredsByTeamID(request.TeamID)
	if err != nil {
		return err
	}

	vk := api.NewVK(vkChannel.AdminAPIKey)

	params := api.Params{
		"owner_id":   -vkChannel.GroupID,
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

	// Публикуем событие об удалении комментария в Kafka
	event := &entity.CommentEvent{
		EventID:    fmt.Sprintf("vk-del-%d-%d", comment.TeamID, comment.ID),
		TeamID:     comment.TeamID,
		PostID:     derefInt(comment.PostUnionID),
		Type:       entity.CommentDeleted,
		CommentID:  comment.ID,
		OccurredAt: time.Now(),
	}
	if err := c.eventRepo.PublishCommentEvent(context.Background(), event); err != nil {
		log.Errorf("Не удалось опубликовать событие об удалении комментария в Kafka: %v", err)
	}

	return nil
}

func derefInt(ptr *int) int {
	if ptr != nil {
		return *ptr
	}
	return 0
}
