package vkontakte

import (
	"context"
	"fmt"
	"io"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"postic-backend/pkg/retry"
	"strings"
	"time"

	"github.com/SevereCloud/vksdk/v3/api"
	"github.com/labstack/gommon/log"
)

type Comment struct {
	commentRepo   repo.Comment
	teamRepo      repo.Team
	uploadUseCase usecase.Upload
	eventRepo     repo.CommentEventRepository
}

func NewVkontakteComment(
	commentRepo repo.Comment,
	teamRepo repo.Team,
	uploadUseCase usecase.Upload,
	eventRepo repo.CommentEventRepository,
) *Comment {
	return &Comment{
		commentRepo:   commentRepo,
		teamRepo:      teamRepo,
		uploadUseCase: uploadUseCase,
		eventRepo:     eventRepo,
	}
}

func (c *Comment) uploadPhoto(vk *api.VK, groupId int, upload *entity.Upload) (string, error) {
	var uploadResponse api.PhotosSaveWallPhotoResponse
	err := retry.Retry(func() error {
		var err error
		upload.RawBytes.Seek(0, io.SeekStart) // Сбрасываем указатель на начало файла на всякий случай
		uploadResponse, err = vk.UploadGroupWallPhoto(groupId, upload.RawBytes)
		return err
	})
	if err != nil {
		return "", err
	}
	if len(uploadResponse) == 0 {
		return "", fmt.Errorf("no photos uploaded")
	}
	return fmt.Sprintf("photo%d_%d", uploadResponse[0].OwnerID, uploadResponse[0].ID), nil
}

func (c *Comment) uploadVideo(vk *api.VK, groupId int, upload *entity.Upload) (string, error) {
	var videoSaveResponse api.VideoSaveResponse
	err := retry.Retry(func() error {
		var err error
		upload.RawBytes.Seek(0, io.SeekStart) // Сбрасываем указатель на начало файла на всякий случай
		videoSaveResponse, err = vk.UploadVideo(api.Params{
			"group_id": groupId,
		}, upload.RawBytes)
		return err
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("video%d_%d", videoSaveResponse.OwnerID, videoSaveResponse.VideoID), nil
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
		upload, err := c.uploadUseCase.GetUpload(attachment)
		if err != nil {
			log.Errorf("Failed to get upload: %v", err)
			return 0, err
		}
		switch upload.FileType {
		case "photo":
			photoAttachment, err := c.uploadPhoto(vk, vkChannel.GroupID, upload)
			if err != nil {
				log.Errorf("Failed to upload photo: %v", err)
				return 0, err
			}
			attachments = append(attachments, photoAttachment)
		case "video":
			videoAttachment, err := c.uploadVideo(vk, vkChannel.GroupID, upload)
			if err != nil {
				log.Errorf("Failed to upload video: %v", err)
				return 0, err
			}
			attachments = append(attachments, videoAttachment)
		default:
			log.Warnf("Unsupported attachment type: %s", upload.FileType)
		}
	}
	if len(attachments) > 0 {
		params["attachments"] = strings.Join(attachments, ",")
	}
	var response api.WallCreateCommentResponse
	err = retry.Retry(func() error {
		var err error
		response, err = vk.WallCreateComment(params)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("failed to reply to VK comment: %w", err)
	}
	// Подготавливаем вложения для сохранения в БД
	commentAttachments := make([]*entity.Upload, 0)
	for _, attachment := range request.Attachments {
		upload := &entity.Upload{
			ID: attachment,
		}
		commentAttachments = append(commentAttachments, upload)
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
		Attachments:       commentAttachments, // Передаем вложения для сохранения в БД
	})
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

	err = retry.Retry(func() error {
		_, err := vk.WallDeleteComment(params)
		return err
	})
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
