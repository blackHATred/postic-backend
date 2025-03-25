package service

import (
	"errors"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"slices"
	"time"
)

type Post struct {
	postRepo        repo.Post
	userRepo        repo.User
	telegramUseCase usecase.Platform
	vkUseCase       usecase.Platform
}

func NewPost(postRepo repo.Post, userRepo repo.User, telegram, vk usecase.Platform) usecase.Post {
	return &Post{
		postRepo:        postRepo,
		userRepo:        userRepo,
		telegramUseCase: telegram,
		vkUseCase:       vk,
	}
}

func (p *Post) GetPostStatus(postID int, platform string) (*entity.GetPostStatusResponse, error) {
	action, err := p.postRepo.GetPostAction(postID, platform, true)
	if err != nil {
		return nil, err
	}
	return &entity.GetPostStatusResponse{
		PostID:     postID,
		Platform:   platform,
		Status:     action.Status,
		ErrMessage: action.ErrMessage,
	}, nil
}

func (p *Post) GetPosts(userID int) ([]*entity.PostUnion, error) {
	return p.postRepo.GetPostsByUserID(userID)
}

func (p *Post) AddPost(request *entity.AddPostRequest) error {
	if len(request.Platforms) == 0 {
		return errors.New("no platforms")
	}
	if len(request.Attachments) == 0 && request.Text == "" {
		return errors.New("no attachments and no text")
	}
	// сначала создаем запись об агрегированном посте
	postUnionID, err := p.postRepo.AddPostUnion(
		&entity.PostUnion{
			Text:        request.Text,
			PubDate:     time.Unix(int64(request.PubTime), 0),
			Attachments: request.Attachments,
			Platforms:   request.Platforms,
			CreatedAt:   time.Now(),
			UserID:      request.UserId,
		},
	)
	if err != nil {
		return err
	}
	// затем создаем действия на публикацию
	if slices.Contains(request.Platforms, "vk") {
		// запускаем подзадачу на публикацию
		// go p.postToVK(postUnionID)
	}
	if slices.Contains(request.Platforms, "tg") {
		// запускаем подзадачу на публикацию
		tgAddPostAction := entity.PostAction{
			PostUnionID: postUnionID,
			Platform:    "tg",
			Status:      "pending",
			ErrMessage:  "",
			CreatedAt:   time.Now(),
		}
		if err = p.telegramUseCase.AddPostInQueue(tgAddPostAction); err != nil {
			return err
		}
	}

	return nil
}

/*
func (p *Post) postToVK(postUnionID int) {
	// создаём новое действие
	postActionID, err := p.postRepo.AddPostActionVK(postUnionID)
	if err != nil {
		return
	}
	// получаем данные поста, который нужно опубликовать
	postUnion, err := p.postRepo.GetPostUnion(postActionID)
	if err != nil {
		return
	}
	// получаем канал публикации
	channel, err := p.postRepo.GetVKChannel(postUnion.UserID)
	if err != nil {
		return
	}
	// публикуем пост
	vk := api.NewVK(channel.APIKey)
	params := api.Params{
		"owner_id": channel.GroupID,
		// сделать attachments!
		"message": postUnion.Text,
		"guid":    postActionID, // чтобы одна и та же запись не опубликовалась дважды (если что-то пойдет не так)
	}
	// если дата публикации в будущем, то указываем ее в формате UNIX timestamp
	if postUnion.PubDate.After(time.Now()) && postUnion.PubDate.Before(time.Now().Add(time.Hour*24*365)) {
		params["publish_date"] = postUnion.PubDate.Unix()
	}
	response, err := vk.WallPost(params)
	if err != nil {
		// обновляем статус
		_ = p.postRepo.EditPostActionVK(postActionID, "error", err.Error())
		return
	}
	// обновляем статус
	_ = p.postRepo.EditPostAction(postActionID, "success", "")
	_ = p.postRepo.AddPostVK(postUnion.ID, response.PostID)
}
*/
