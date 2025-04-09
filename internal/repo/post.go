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

	// GetScheduledPosts возвращает список запланированных постов по статусу
	GetScheduledPosts(status string) ([]*entity.ScheduledPost, error)
	// GetScheduledPost возвращает запланированный пост по ID
	GetScheduledPost(postUnionID int) (*entity.ScheduledPost, error)
	// AddScheduledPost добавляет запланированный пост и возвращает его айди
	AddScheduledPost(scheduledPost *entity.ScheduledPost) (int, error)
	// EditScheduledPost редактирует запись о запланированном посте
	EditScheduledPost(scheduledPost *entity.ScheduledPost) error
	// DeleteScheduledPost удаляет запланированный пост
	DeleteScheduledPost(postUnionID int) error

	// GetPostAction возвращает действие по ID
	GetPostAction(postActionID int) (*entity.PostAction, error)
	// AddPostAction добавляет действие к посту и возвращает его айди
	AddPostAction(postAction *entity.PostAction) (int, error)
	// EditPostAction редактирует действие
	EditPostAction(postAction *entity.PostAction) error

	// GetPostPlatform возвращает пост с платформы по ID поста на этой платформе
	GetPostPlatform(postPlatformID int, platform string) (*entity.PostPlatform, error)
	// AddPostPlatform добавляет связанную с PostUnion запись про пост, опубликованный на платформе
	AddPostPlatform(postPlatform *entity.PostPlatform) (int, error)
	DeletePostPlatform() error
}
