package repo

import (
	"postic-backend/internal/entity"
)

type Post interface {
	// GetPostUnion возвращает агрегированный пост
	GetPostUnion(postUnionID int) (*entity.PostUnion, error)
	// AddPostUnion добавляет агрегированный пост и возвращает его айди
	AddPostUnion(*entity.PostUnion) (int, error)
	// EditPostUnion редактирует агрегированный пост
	EditPostUnion(*entity.PostUnion) error

	GetPostAction()
	AddPostAction() (int, error)
	EditPostAction() error

	GetPostPlatform()
	AddPostPlatform() (int, error)
	DeletePostPlatform() error
}
