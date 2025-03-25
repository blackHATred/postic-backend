package usecase

import "postic-backend/internal/entity"

type Post interface {
	// AddPost добавляет агрегированный пост
	AddPost(request *entity.AddPostRequest) error
	// GetPosts возвращает список агрегированных постов
	GetPosts(userID int) ([]*entity.PostUnion, error)
	// GetPostStatus возвращает статус публикации поста
	GetPostStatus(postID int, platform string) (*entity.GetPostStatusResponse, error)
}
