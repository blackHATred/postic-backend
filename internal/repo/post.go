package repo

import (
	"errors"
	"postic-backend/internal/entity"
	"time"
)

type Post interface {
	// GetPostUnions возвращает агрегированные посты команды с учетом оффсета (ДО указанного момента)
	GetPostUnions(teamID int, offset time.Time, before bool, limit int, filter *string) ([]*entity.PostUnion, error)
	// GetPostUnion возвращает агрегированный пост
	GetPostUnion(postUnionID int) (*entity.PostUnion, error)
	// AddPostUnion добавляет агрегированный пост и возвращает его айди
	AddPostUnion(*entity.PostUnion) (int, error)
	// EditPostUnion редактирует агрегированный пост
	EditPostUnion(*entity.PostUnion) error
	// DeletePlatformFromPostUnion удаляет платформу из PostUnion. Если не остается ни одного platform,
	// то удаляет PostUnion
	DeletePlatformFromPostUnion(postUnionID int, platform string) error

	// GetScheduledPosts возвращает список запланированных постов по статусу и оффсету времени (ДО указанного момента)
	GetScheduledPosts(status string, offset time.Time, before bool, limit int) ([]*entity.ScheduledPost, error)
	// GetScheduledPost возвращает запланированный пост по ID
	GetScheduledPost(postUnionID int) (*entity.ScheduledPost, error)
	// AddScheduledPost добавляет запланированный пост и возвращает его айди
	AddScheduledPost(scheduledPost *entity.ScheduledPost) (int, error)
	// EditScheduledPost редактирует запись о запланированном посте
	EditScheduledPost(scheduledPost *entity.ScheduledPost) error
	// DeleteScheduledPost удаляет запланированный пост
	DeleteScheduledPost(postUnionID int) error

	// GetPostActions возвращает список id действий по ID поста
	GetPostActions(postUnionID int) ([]int, error)
	// GetPostAction возвращает действие по ID
	GetPostAction(postActionID int) (*entity.PostAction, error)
	// AddPostAction добавляет действие к посту и возвращает его айди
	AddPostAction(postAction *entity.PostAction) (int, error)
	// EditPostAction редактирует действие
	EditPostAction(postAction *entity.PostAction) error

	// GetPostPlatform возвращает пост с платформы по ID поста
	GetPostPlatform(postUnionID int, platform string) (*entity.PostPlatform, error)
	// GetPostPlatformByPost возвращает пост с платформы по ID поста и каналу поста
	GetPostPlatformByPost(platformID int, channelID int, platform string) (*entity.PostPlatform, error)
	// AddPostPlatform добавляет связанную с PostUnion запись про пост, опубликованный на платформе
	AddPostPlatform(postPlatform *entity.PostPlatform) (int, error)
	DeletePostPlatform() error
}

var (
	ErrPostPlatformNotFound = errors.New("post platform not found")
	ErrPostUnionNotFound    = errors.New("post union not found")
)
