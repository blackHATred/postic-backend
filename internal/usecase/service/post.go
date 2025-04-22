package service

import (
	"errors"
	"github.com/labstack/gommon/log"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"slices"
	"strings"
	"time"
)

type PostUnion struct {
	postRepo   repo.Post
	teamRepo   repo.Team
	uploadRepo repo.Upload
	telegram   usecase.PostPlatform
}

func NewPostUnion(
	postRepo repo.Post,
	teamRepo repo.Team,
	uploadRepo repo.Upload,
	telegram usecase.PostPlatform,
) usecase.PostUnion {
	p := &PostUnion{
		postRepo:   postRepo,
		teamRepo:   teamRepo,
		uploadRepo: uploadRepo,
		telegram:   telegram,
	}
	// запускаем горутину для мониторинга запланированных постов
	go p.scheduleListen()
	return p
}

func (p *PostUnion) scheduleListen() {
	// мониторим запланированные посты раз в 10 секунд
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// получаем все запланированные посты, которые ждут публикации
			scheduledPosts, err := p.postRepo.GetScheduledPosts("pending", time.Now(), true, 5)
			if err != nil {
				log.Errorf("error getting scheduled posts: %v", err)
				continue
			}
			for _, scheduledPost := range scheduledPosts {
				log.Infof("scheduled posts: %v", *scheduledPost)
				if time.Now().After(scheduledPost.ScheduledAt) {
					// получаем необходимый пост
					postUnion, err := p.postRepo.GetPostUnion(scheduledPost.PostUnionID)
					log.Infof("scheduled post: %v", *postUnion)
					if err != nil {
						log.Errorf("error getting post union: %v", err)
					}
					// публикуем пост
					for _, platform := range postUnion.Platforms {
						switch platform {
						case "tg":
							// добавляем пост в телеграм
							_, err = p.telegram.AddPost(postUnion)
							if err != nil {
								log.Errorf("error adding post to telegram: %v", err)
								continue
							}
							// другие платформы todo
						}
					}
					// обновляем запись о запланированном посте
					scheduledPost.Status = "published"
					err = p.postRepo.EditScheduledPost(scheduledPost)
					if err != nil {
						log.Errorf("error updating scheduled post: %v", err)
						continue
					}
				}
			}
		}
	}
}

func (p *PostUnion) AddPostUnion(request *entity.AddPostRequest) (int, []int, error) {
	if err := request.IsValid(); err != nil {
		return 0, nil, err
	}
	// Проверяем, что пользователь админ или имеет отдельное право на публикации
	permissions, err := p.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return 0, nil, err
	}
	if !slices.Contains(permissions, repo.AdminRole) && !slices.Contains(permissions, repo.PostsRole) {
		return 0, nil, errors.New("user has no permission to create post")
	}

	// Создание записи в таблице post_union
	if request.PubDateTime != nil && request.PubDateTime.After(time.Now().Add(time.Hour*24*365)) {
		return 0, nil, errors.New("publication date is too far in the future")
	}
	attachments := make([]*entity.Upload, len(request.Attachments))
	if len(request.Attachments) > 0 {
		for i, attachment := range request.Attachments {
			upload, err := p.uploadRepo.GetUploadInfo(attachment)
			if err != nil {
				return 0, nil, err
			}
			attachments[i] = upload
		}
	}
	postUnion := &entity.PostUnion{
		UserID:      request.UserID,
		TeamID:      request.TeamID,
		Text:        request.Text,
		Platforms:   request.Platforms,
		CreatedAt:   time.Now(),
		PubDate:     request.PubDateTime,
		Attachments: attachments,
	}
	postUnionID, err := p.postRepo.AddPostUnion(postUnion)
	if err != nil {
		return 0, nil, err
	}
	postUnion.ID = postUnionID

	// Если pubdatetime > now, то создаем запланированную публикацию
	if request.PubDateTime != nil && request.PubDateTime.After(time.Now()) {
		_, err = p.postRepo.AddScheduledPost(&entity.ScheduledPost{
			PostUnionID: postUnionID,
			ScheduledAt: *request.PubDateTime,
			Status:      "pending",
			CreatedAt:   time.Now(),
		})
		// Так как никаких действий с внешними платформами пока не произошло, то возвращаем пустой список actions
		return postUnionID, []int{}, err
	}
	// Если pubdatetime <= now, то на каждой из платформ создаем action
	var actionIDs []int
	for _, platform := range request.Platforms {
		switch platform {
		case "tg":
			platformID, err := p.telegram.AddPost(postUnion)
			if err != nil {
				return postUnionID, actionIDs, err
			}
			actionIDs = append(actionIDs, platformID)
			// todo другие платформы
		}
	}
	return postUnionID, actionIDs, nil
}

func (p *PostUnion) EditPostUnion(request *entity.EditPostRequest) ([]int, error) {
	// редактировать можно только текст, неопубликованные посты, а также посты, с момента публикации которых
	// прошло не более суток

	// проверяем права пользователя
	permissions, err := p.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(permissions, repo.AdminRole) && !slices.Contains(permissions, repo.PostsRole) {
		return nil, usecase.ErrUserForbidden
	}
	// проверяем, что пост принадлежит этой команде
	postUnion, err := p.postRepo.GetPostUnion(request.PostUnionID)
	if err != nil {
		return nil, err
	}
	if postUnion.TeamID != request.TeamID {
		return nil, usecase.ErrUserForbidden
	}
	if (postUnion.PubDate != nil && time.Now().After(postUnion.PubDate.Add(time.Hour*24))) ||
		(postUnion.PubDate == nil && time.Now().After(postUnion.CreatedAt.Add(time.Hour*24))) {
		return nil, usecase.ErrPostUnavailableToEdit
	}
	if len(postUnion.Attachments) == 0 && strings.TrimSpace(request.Text) == "" {
		return nil, usecase.ErrPostTextAndAttachmentsAreRequired
	}
	// если это запланированный и пока что неопубликованный пост, то просто редактируем его
	if postUnion.PubDate != nil {
		postUnion.Text = request.Text
		err = p.postRepo.EditPostUnion(postUnion)
		if err != nil {
			return nil, err
		}
		// новых action не произошло, поэтому возвращаем пустой слайс
		return []int{}, nil
	}
	// если это уже опубликованный пост, то создаем новый action на редактирование на всех платформах
	var actionIDs []int
	for _, platform := range postUnion.Platforms {
		switch platform {
		case "tg":
			actionID, err := p.telegram.EditPost(&entity.EditPostRequest{
				PostUnionID: request.PostUnionID,
				Text:        request.Text,
			})
			if err != nil {
				return nil, err
			}
			actionIDs = append(actionIDs, actionID)
			// todo другие платформы
		}
	}
	// обновляем текст поста в базе данных
	postUnion.Text = request.Text
	err = p.postRepo.EditPostUnion(postUnion)
	if err != nil {
		return nil, err
	}
	return actionIDs, nil
}

func (p *PostUnion) DeletePostUnion(request *entity.DeletePostRequest) ([]int, error) {
	// проверяем права пользователя
	permissions, err := p.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(permissions, repo.AdminRole) && !slices.Contains(permissions, repo.PostsRole) {
		return nil, usecase.ErrUserForbidden
	}
	// проверяем, что пост принадлежит этой команде
	postUnion, err := p.postRepo.GetPostUnion(request.PostUnionID)
	if err != nil {
		return nil, err
	}
	if postUnion.TeamID != request.TeamID {
		return nil, usecase.ErrUserForbidden
	}
	var actionIDs []int
	for _, platform := range postUnion.Platforms {
		switch platform {
		case "tg":
			actionID, err := p.telegram.DeletePost(&entity.DeletePostRequest{
				UserID:      request.UserID,
				TeamID:      request.TeamID,
				PostUnionID: request.PostUnionID,
			})
			if err != nil {
				return nil, err
			}
			actionIDs = append(actionIDs, actionID)
			// todo другие платформы
		}
	}
	return actionIDs, nil
}

func (p *PostUnion) GetPostUnion(request *entity.GetPostRequest) (*entity.PostUnion, error) {
	// проверяем права пользователя
	permissions, err := p.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(permissions, repo.AdminRole) && !slices.Contains(permissions, repo.PostsRole) {
		return nil, usecase.ErrUserForbidden
	}
	// проверяем, что пост принадлежит этой команде
	postUnion, err := p.postRepo.GetPostUnion(request.PostUnionID)
	if err != nil {
		return nil, err
	}
	if postUnion.TeamID != request.TeamID {
		return nil, usecase.ErrUserForbidden
	}
	return postUnion, nil
}

func (p *PostUnion) GetPosts(request *entity.GetPostsRequest) ([]*entity.PostUnion, error) {
	// проверяем права пользователя
	permissions, err := p.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(permissions, repo.AdminRole) && !slices.Contains(permissions, repo.PostsRole) {
		return nil, usecase.ErrUserForbidden
	}
	if request.Limit > 100 {
		request.Limit = 100
	}
	// получаем посты
	offset := time.Now()
	if request.Offset != nil {
		offset = *request.Offset
	}
	posts, err := p.postRepo.GetPostUnions(request.TeamID, offset, request.Before, request.Limit, request.Filter)
	if err != nil {
		return nil, err
	}
	return posts, nil
}

func (p *PostUnion) GetPostStatus(request *entity.PostStatusRequest) ([]*entity.PostActionResponse, error) {
	// проверяем права пользователя
	permissions, err := p.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(permissions, repo.AdminRole) && !slices.Contains(permissions, repo.PostsRole) {
		return nil, usecase.ErrUserForbidden
	}
	// проверяем, что пост принадлежит этой команде
	postUnion, err := p.postRepo.GetPostUnion(request.PostUnionID)
	if err != nil {
		return nil, err
	}
	if postUnion.TeamID != request.TeamID {
		return nil, usecase.ErrUserForbidden
	}

	actionIDs, err := p.postRepo.GetPostActions(request.PostUnionID)
	if err != nil {
		return nil, err
	}
	responses := make([]*entity.PostActionResponse, len(actionIDs))

	for i, actionID := range actionIDs {
		action, err := p.postRepo.GetPostAction(actionID)
		if err != nil {
			return nil, err
		}
		responses[i] = &entity.PostActionResponse{
			PostID:     request.PostUnionID,
			Platform:   action.Platform,
			Operation:  action.Operation,
			Status:     action.Status,
			ErrMessage: action.ErrMessage,
			CreatedAt:  action.CreatedAt,
		}
	}

	return responses, nil
}

func (p *PostUnion) DoAction(request *entity.DoActionRequest) (int, error) {
	// проверяем права пользователя
	permissions, err := p.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return 0, err
	}
	if !slices.Contains(permissions, repo.AdminRole) && !slices.Contains(permissions, repo.PostsRole) {
		return 0, usecase.ErrUserForbidden
	}
	// проверяем, что пост принадлежит этой команде
	postUnion, err := p.postRepo.GetPostUnion(request.PostUnionID)
	if err != nil {
		return 0, err
	}
	if postUnion.TeamID != request.TeamID {
		return 0, usecase.ErrUserForbidden
	}

	switch request.Operation {
	case "add":
		switch request.Platform {
		case "tg":
			actionID, err := p.telegram.AddPost(postUnion)
			if err != nil {
				return 0, err
			}
			return actionID, nil
		}
	case "delete":
		switch request.Platform {
		case "tg":
			actionID, err := p.telegram.DeletePost(&entity.DeletePostRequest{
				UserID:      request.UserID,
				TeamID:      request.TeamID,
				PostUnionID: request.PostUnionID,
			})
			if err != nil {
				return 0, err
			}
			return actionID, nil
		}
	case "edit":
		switch request.Platform {
		case "tg":
			actionID, err := p.telegram.EditPost(&entity.EditPostRequest{
				UserID:      request.UserID,
				TeamID:      request.TeamID,
				PostUnionID: request.PostUnionID,
				Text:        postUnion.Text,
			})
			if err != nil {
				return 0, err
			}
			return actionID, nil
		}
	}

	return 0, nil
}
