package vkontakte

import (
	"errors"
	"fmt"
	"github.com/SevereCloud/vksdk/v3/api"
	"github.com/labstack/gommon/log"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"postic-backend/pkg/retry"
	"strings"
	"time"
)

type Post struct {
	postRepo   repo.Post
	teamRepo   repo.Team
	uploadRepo repo.Upload
}

func NewPost(
	postRepo repo.Post,
	teamRepo repo.Team,
	uploadRepo repo.Upload,
) usecase.PostPlatform {
	return &Post{
		postRepo:   postRepo,
		teamRepo:   teamRepo,
		uploadRepo: uploadRepo,
	}
}

func (p *Post) AddPost(request *entity.PostUnion) (int, error) {
	// Create post action record
	actionId, err := p.createPostAction(request)
	if err != nil {
		return 0, err
	}

	// Process the post asynchronously
	go p.publishPost(request, actionId)

	return actionId, nil
}

func (p *Post) createPostAction(request *entity.PostUnion) (int, error) {
	var postActionId int
	err := retry.Retry(func() error {
		var err error
		postActionId, err = p.postRepo.AddPostAction(&entity.PostAction{
			PostUnionID: request.ID,
			Operation:   "publish",
			Platform:    "vk",
			Status:      "pending",
			CreatedAt:   time.Now(),
		})
		return err
	})
	return postActionId, err
}

func (p *Post) updatePostActionStatus(actionId int, status, errMsg string) {
	err := retry.Retry(func() error {
		action, err := p.postRepo.GetPostAction(actionId)
		if err != nil {
			log.Errorf("error getting post action: %v", err)
			return err
		}
		return p.postRepo.EditPostAction(&entity.PostAction{
			ID:          actionId,
			PostUnionID: action.PostUnionID,
			Operation:   action.Operation,
			Platform:    action.Platform,
			Status:      status,
			ErrMessage:  errMsg,
			CreatedAt:   action.CreatedAt,
		})
	})
	if err != nil {
		log.Errorf("error while updating post action status: %v", err)
	}
}

func (p *Post) publishPost(request *entity.PostUnion, actionId int) {
	// Get VK credentials
	groupId, adminApiKey, _, err := p.teamRepo.GetVKCredsByTeamID(request.TeamID)
	if err != nil {
		p.updatePostActionStatus(actionId, "error", err.Error())
		return
	}

	// Initialize VK API with admin API key
	vk := api.NewVK(adminApiKey)

	// Prepare parameters for wall post
	params := api.Params{
		"owner_id":   -groupId, // Negative ID for groups
		"message":    request.Text,
		"from_group": 1, // 1 means post on behalf of the group
	}

	// Handle attachments
	if len(request.Attachments) > 0 {
		attachmentsStr, err := p.uploadAttachments(vk, groupId, request.Attachments)
		if err != nil {
			p.updatePostActionStatus(actionId, "error", err.Error())
			return
		}

		if attachmentsStr != "" {
			params["attachments"] = attachmentsStr
		}
	}

	// Post to VK wall
	var response api.WallPostResponse
	err = retry.Retry(func() error {
		response, err = vk.WallPost(params)
		return err
	})

	if err != nil {
		p.updatePostActionStatus(actionId, "error", err.Error())
		return
	}

	// Save post ID to our database
	err = retry.Retry(func() error {
		_, err := p.postRepo.AddPostPlatform(&entity.PostPlatform{
			PostUnionId: request.ID,
			PostId:      response.PostID,
			Platform:    "vk",
		})
		return err
	})

	if err != nil {
		log.Errorf("error while adding post platform: %v", err)
	}

	p.updatePostActionStatus(actionId, "success", "")
}

func (p *Post) uploadAttachments(vk *api.VK, groupId int, attachments []*entity.Upload) (string, error) {
	var attachmentStrings []string

	for _, attachment := range attachments {
		upload, err := p.uploadRepo.GetUpload(attachment.ID)
		if err != nil {
			return "", err
		}

		switch attachment.FileType {
		case "photo":
			photoAttachment, err := p.uploadPhoto(vk, groupId, upload)
			if err != nil {
				return "", err
			}
			attachmentStrings = append(attachmentStrings, photoAttachment)

		case "video":
			videoAttachment, err := p.uploadVideo(vk, groupId, upload)
			if err != nil {
				return "", err
			}
			attachmentStrings = append(attachmentStrings, videoAttachment)
		}
	}

	return strings.Join(attachmentStrings, ","), nil
}

func (p *Post) uploadPhoto(vk *api.VK, groupId int, upload *entity.Upload) (string, error) {
	// Upload photo to VK server
	uploadResponse, err := vk.UploadGroupWallPhoto(groupId, upload.RawBytes)
	if err != nil {
		return "", err
	}

	if len(uploadResponse) == 0 {
		return "", errors.New("no photos uploaded")
	}

	// Format: photo{owner_id}_{media_id}
	photoAttachment := fmt.Sprintf("photo%d_%d", uploadResponse[0].OwnerID, uploadResponse[0].ID)
	return photoAttachment, nil
}

func (p *Post) uploadVideo(vk *api.VK, groupId int, upload *entity.Upload) (string, error) {
	// Загрузить видео на сервер VK
	videoSaveResponse, err := vk.UploadVideo(api.Params{
		"group_id": groupId,
	}, upload.RawBytes)
	if err != nil {
		return "", fmt.Errorf("failed to upload video: %w", err)
	}

	// Форматировать идентификатор видео: video{owner_id}_{video_id}
	videoAttachment := fmt.Sprintf("video%d_%d", videoSaveResponse.OwnerID, videoSaveResponse.VideoID)
	return videoAttachment, nil
}

func (p *Post) EditPost(request *entity.EditPostRequest) (int, error) {
	// Create post action record for edit operation
	var postActionId int
	err := retry.Retry(func() error {
		var err error
		postActionId, err = p.postRepo.AddPostAction(&entity.PostAction{
			PostUnionID: request.PostUnionID,
			Operation:   "edit",
			Platform:    "vk",
			Status:      "pending",
			CreatedAt:   time.Now(),
		})
		return err
	})
	if err != nil {
		return 0, err
	}

	// Get post information
	post, err := p.postRepo.GetPostUnion(request.PostUnionID)
	if err != nil {
		p.updatePostActionStatus(postActionId, "error", err.Error())
		return 0, err
	}

	// Get VK post information
	postPlatform, err := p.postRepo.GetPostPlatform(request.PostUnionID, "vk")
	if err != nil {
		p.updatePostActionStatus(postActionId, "error", err.Error())
		return 0, err
	}

	// Start asynchronous edit operation
	go p.editPostAsync(post, postActionId, postPlatform.PostId, request.Text)

	return postActionId, nil
}

func (p *Post) editPostAsync(post *entity.PostUnion, actionId, postId int, newText string) {
	// Get VK credentials
	groupId, adminApiKey, _, err := p.teamRepo.GetVKCredsByTeamID(post.TeamID)
	if err != nil {
		p.updatePostActionStatus(actionId, "error", err.Error())
		return
	}

	// Initialize VK API with admin API key
	vk := api.NewVK(adminApiKey)

	// Prepare parameters for editing the post
	params := api.Params{
		"owner_id": -groupId, // Negative ID for groups
		"post_id":  postId,
		"message":  newText,
	}

	// If post has attachments, we need to maintain them
	if len(post.Attachments) > 0 {
		attachmentsStr, err := p.uploadAttachments(vk, groupId, post.Attachments)
		if err != nil {
			p.updatePostActionStatus(actionId, "error", err.Error())
			return
		}

		if attachmentsStr != "" {
			params["attachments"] = attachmentsStr
		}
	}

	// Edit the post
	err = retry.Retry(func() error {
		_, err := vk.WallEdit(params)
		return err
	})

	if err != nil {
		p.updatePostActionStatus(actionId, "error", err.Error())
		return
	}

	p.updatePostActionStatus(actionId, "success", "")
}

func (p *Post) DeletePost(request *entity.DeletePostRequest) (int, error) {
	// Create post action record for delete operation
	var postActionId int
	err := retry.Retry(func() error {
		var err error
		postActionId, err = p.postRepo.AddPostAction(&entity.PostAction{
			PostUnionID: request.PostUnionID,
			Operation:   "delete",
			Platform:    "vk",
			Status:      "pending",
			CreatedAt:   time.Now(),
		})
		return err
	})
	if err != nil {
		return 0, err
	}

	// Get post information
	post, err := p.postRepo.GetPostUnion(request.PostUnionID)
	if err != nil {
		p.updatePostActionStatus(postActionId, "error", err.Error())
		return 0, err
	}

	// Start asynchronous delete operation
	go p.deletePostAsync(post, postActionId)

	return postActionId, nil
}

func (p *Post) deletePostAsync(post *entity.PostUnion, actionId int) {
	// Get VK post information
	postPlatform, err := p.postRepo.GetPostPlatform(post.ID, "vk")
	if err != nil {
		p.updatePostActionStatus(actionId, "error", err.Error())
		return
	}

	// Get VK credentials
	groupId, adminApiKey, _, err := p.teamRepo.GetVKCredsByTeamID(post.TeamID)
	if err != nil {
		p.updatePostActionStatus(actionId, "error", err.Error())
		return
	}

	// Initialize VK API with admin API key
	vk := api.NewVK(adminApiKey)

	// Delete the post
	err = retry.Retry(func() error {
		_, err := vk.WallDelete(api.Params{
			"owner_id": -groupId, // Negative ID for groups
			"post_id":  postPlatform.PostId,
		})
		return err
	})

	if err != nil {
		p.updatePostActionStatus(actionId, "error", err.Error())
		return
	}

	// Remove post from our platform
	err = retry.Retry(func() error {
		return p.postRepo.DeletePlatformFromPostUnion(post.ID, "vk")
	})

	if err != nil {
		log.Errorf("error while deleting post platform: %v", err)
	}

	p.updatePostActionStatus(actionId, "success", "")
}
