package usecase

import "postic-backend/internal/entity"

type Post interface {
	// AddVKChannel добавляет группу ВКонтакте как канал публикации для пользователя
	AddVKChannel(userID int, groupID int, apiKey string) error
	// AddPost добавляет агрегированный пост
	AddPost(request *entity.AddPostRequest) error
	// GetPosts возвращает список агрегированных постов
	GetPosts(userID int) ([]*entity.PostUnion, error)
	// GetPostStatus возвращает статус публикации поста
	GetPostStatus(postID int) ([]*entity.GetPostStatusResponse, error)
}
