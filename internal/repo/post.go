package repo

import (
	"postic-backend/internal/entity"
)

type Post interface {
	// PostUnionId

	// GetPostsByUserID возвращает список агрегированных постов по ID пользователя
	GetPostsByUserID(userID int) ([]*entity.PostUnion, error)
	// GetPostUnion возвращает агрегированный пост
	GetPostUnion(postUnionID int) (*entity.PostUnion, error)
	// GetPostUnions возвращает список агрегированных постов
	GetPostUnions(userID int) ([]*entity.PostUnion, error)
	// AddPostUnion добавляет агрегированный пост
	AddPostUnion(*entity.PostUnion) (int, error)

	// AddPostAction

	// GetPostAction возвращает действие на создание поста
	GetPostAction(postUnionID int, platform string, last bool) (*entity.AddPostAction, error)
	// AddPostAction добавляет действие на создание поста
	AddPostAction(*entity.AddPostAction) (int, error)
	// EditPostActionStatus изменяет статус действия на создание поста
	EditPostActionStatus(postUnionID int, status, errorMessage string) error

	// Platforms

	// GetLastUpdateTG возвращает id последнего обработанного event в telegram
	GetLastUpdateTG() (int, error)
	// SetLastUpdateTG устанавливает id последнего обработанного event в telegram
	SetLastUpdateTG(updateID int) error
	// AddPostVK добавляет пост в ВК
	AddPostVK(postUnionID, postID int) error
	// AddPostTG добавляет пост в Телеграм
	AddPostTG(postUnionID, postID int) error
	// GetPostTGByMessageID возвращает пост в Телеграм по его идентификатору
	GetPostTGByMessageID(messageID int) (*entity.PostTG, error)
}
