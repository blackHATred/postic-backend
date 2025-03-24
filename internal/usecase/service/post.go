package service

import (
	"errors"
	"github.com/SevereCloud/vksdk/v3/api"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"slices"
	"time"
)

type Post struct {
	postRepo repo.Post
}

func NewPost(postRepo repo.Post) usecase.Post {
	return &Post{
		postRepo: postRepo,
	}
}

func (p *Post) AddVKChannel(userID int, groupID int, apiKey string) error {
	// пока что без какой-либо глубокой логики
	return p.postRepo.PutChannel(userID, groupID, apiKey)
}

func (p *Post) GetPostStatus(postID int) ([]*entity.GetPostStatusResponse, error) {
	status, err := p.postRepo.GetPostStatusVKTG(postID)
	if err != nil {
		return nil, errors.New("get post status failed")
	}
	return []*entity.GetPostStatusResponse{status}, nil
}

func (p *Post) GetPosts(userID int) ([]*entity.PostUnion, error) {
	return p.postRepo.GetPosts(userID)
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
		go p.postToVK(postUnionID)
	}
	if slices.Contains(request.Platforms, "tg") {
		// запускаем подзадачу на публикацию
		// todo
	}

	return nil
}

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
	_ = p.postRepo.EditPostActionVK(postActionID, "success", "")
	_ = p.postRepo.AddPostVK(postUnion.ID, response.PostID)
}
