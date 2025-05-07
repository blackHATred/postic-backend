package repo

import (
	"errors"
	"postic-backend/internal/entity"
	"time"
)

type Analytics interface {
	// GetPostPlatformStatsByPostUnionID возвращает статистику по посту, используя ID поста
	GetPostPlatformStatsByPostUnionID(postUnionID int, platform string) (*entity.PostPlatformStats, error)
	// GetPostPlatformStatsByPeriod возвращает статистику по постам, используя период времени публикации постов
	GetPostPlatformStatsByPeriod(startDate, endDate time.Time, platform string) ([]*entity.PostPlatformStats, error)
	// EditPostPlatformStats обновляет статистику по посту и платформе
	EditPostPlatformStats(stats *entity.PostPlatformStats) error
	SetPostViewsCount(postUnionID int, platform string, postViewsCount int) error
	// AddPostPlatformStats добавляет статистику по посту и платформе
	AddPostPlatformStats(stats *entity.PostPlatformStats) error
	// CommentsCount возвращает количество комментариев к посту
	CommentsCount(postUnionID int) (int, error)
}

var (
	ErrPostPlatformStatsNotFound = errors.New("post platform stats not found")
)
