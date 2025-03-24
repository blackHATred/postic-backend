package repo

import (
	"postic-backend/internal/entity"
)

type Post interface {
	// PutChannel добавляет группу ВКонтакте как канал публикации для пользователя
	PutChannel(userID, groupID int, apiKey string) error
	// GetVKChannel возвращает группу ВКонтакте как канал публикации для пользователя
	GetVKChannel(userID int) (*entity.VKChannel, error)

	// AddPostUnion добавляет агрегированный пост
	AddPostUnion(*entity.PostUnion) (int, error)
	// GetPostUnion возвращает агрегированный пост
	GetPostUnion(postUnionID int) (*entity.PostUnion, error)

	// GetPostActionVK возвращает действие на создание поста в ВК
	GetPostActionVK(postUnionID int) (*entity.PostActionVK, error)
	// AddPostActionVK добавляет действие на создание поста в ВК
	AddPostActionVK(postUnionID int) (int, error)
	// EditPostActionVK изменяет действие на создание поста в ВК
	EditPostActionVK(postUnionID int, status, errorMessage string) error
	// AddPostVK добавляет пост в ВК
	AddPostVK(postUnionID, postID int) error
	// GetPosts возвращает список агрегированных постов
	GetPosts(userID int) ([]*entity.PostUnion, error)

	GetPostStatusVKTG(postID int) (*entity.GetPostStatusResponse, error)
}
