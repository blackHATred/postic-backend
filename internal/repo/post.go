package repo

import (
	"postic-backend/internal/entity"
)

type Post interface {
	// PostUnion

	// GetPostsByUserID возвращает список агрегированных постов по ID пользователя
	GetPostsByUserID(userID int) ([]*entity.PostUnion, error)
	// GetPostUnion возвращает агрегированный пост
	GetPostUnion(postUnionID int) (*entity.PostUnion, error)
	// GetPostUnions возвращает список агрегированных постов
	GetPostUnions(userID int) ([]*entity.PostUnion, error)
	// AddPostUnion добавляет агрегированный пост
	AddPostUnion(*entity.PostUnion) (int, error)

	// PostAction

	// GetPostAction возвращает действие на создание поста
	GetPostAction(postUnionID int, platform string, last bool) (*entity.PostAction, error)
	// AddPostAction добавляет действие на создание поста
	AddPostAction(*entity.PostAction) (int, error)
	// EditPostActionStatus изменяет статус действия на создание поста
	EditPostActionStatus(postUnionID int, status, errorMessage string) error

	// Platform Posts

	// AddPostVK добавляет пост в ВК
	AddPostVK(postUnionID, postID int) error
	// AddPostTG добавляет пост в Телеграм
	AddPostTG(postUnionID, postID int) error
}
