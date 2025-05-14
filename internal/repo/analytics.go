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
	GetPostPlatformStatsByPeriod(startDate, endDate time.Time, platform string) (*entity.PostPlatformStats, error)
	// CommentsCount возвращает количество комментариев к посту
	CommentsCount(postUnionID int) (int, error)
	// CreateNewPeriod создает новый период для аналитики
	CreateNewPeriod(postUnionID int, platform string) error
	// EndPeriod завершает текущий период для аналитики
	EndPeriod(postUnionID int, platform string) error
	// UpdateLastPlatformStats обновляет последний период актуальной аналитикой или создаёт её, если ни одного периода нет
	UpdateLastPlatformStats(stats *entity.PostPlatformStats, platform string) error

	// GetUserKPI возвращает KPI по посту
	GetUserKPI(userID int, startDate, endDate time.Time) (*entity.UserKPI, error)
	// CompareUserKPI возвращает KPI по постам для нескольких пользователей
	CompareUserKPI(userIDs []int, startDate, endDate time.Time) (map[int]*entity.UserKPI, error)
}

var (
	ErrPostPlatformStatsNotFound = errors.New("post platform stats not found")
)
