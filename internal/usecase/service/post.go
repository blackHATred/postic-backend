package service

import (
	"errors"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"slices"
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
	// todo: запустить отдельную горутину, которая по тикеру будет смотреть запланированные публикации и осуществлять их
	return &PostUnion{
		postRepo:   postRepo,
		teamRepo:   teamRepo,
		uploadRepo: uploadRepo,
		telegram:   telegram,
	}
}

func (p *PostUnion) AddPostUnion(request *entity.AddPostRequest) (int, []int, error) {
	// Проверяем, что пользователь админ или имеет отдельное право на публикации
	permissions, err := p.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return 0, nil, err
	}
	if !slices.Contains(permissions, repo.AdminRole) && !slices.Contains(permissions, repo.PostsRole) {
		return 0, nil, errors.New("user has no permission to create post")
	}

	// Создание записи в таблице post_union
	pubDateTime := time.Unix(int64(request.PubDateTime), 0).UTC()
	if pubDateTime.After(time.Now().Add(time.Hour * 24 * 365)) {
		return 0, nil, errors.New("publication date is too far in the future")
	}
	// Возможен случай, когда pubDateTime <= now, тогда пост будет опубликован сразу без ошибки
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
		PubDate:     &pubDateTime,
		Attachments: attachments,
	}

	postUnionID, err := p.postRepo.AddPostUnion(postUnion)
	if err != nil {
		return 0, nil, err
	}
	postUnion.ID = postUnionID

	// Если pubdatetime > now, то создаем запланированную публикацию
	if pubDateTime.After(time.Now()) {
		_, err = p.postRepo.AddScheduledPost(&entity.ScheduledPost{
			PostUnionID: postUnionID,
			ScheduledAt: pubDateTime,
			Status:      "pending",
			CreatedAt:   time.Now(),
		})
		// Так как никаких действий с внешними платформами пока не произошло, то возвращаем пустой список actions
		return postUnionID, []int{}, nil
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
	//TODO implement me
	panic("implement me")
}

func (p *PostUnion) DeletePostUnion(request *entity.DeletePostRequest) ([]int, error) {
	//TODO implement me
	panic("implement me")
}

func (p *PostUnion) GetPostUnion(request *entity.GetPostRequest) (*entity.PostUnion, error) {
	//TODO implement me
	panic("implement me")
}

func (p *PostUnion) GetPosts(request *entity.GetPostsRequest) ([]*entity.PostUnion, error) {
	//TODO implement me
	panic("implement me")
}

func (p *PostUnion) GetPostStatus(request *entity.PostStatusRequest) ([]*entity.PostActionResponse, error) {
	//TODO implement me
	panic("implement me")
}
