package repo

import (
	"errors"
	"postic-backend/internal/entity"
	"time"
)

type Analytics interface {
	// GetPostPlatformStatsByPostUnionID возвращает последнюю статистику по посту
	GetPostPlatformStatsByPostUnionID(postUnionID int, platform string) (*entity.PostPlatformStats, error)
	// GetPostPlatformStatsByDateRange возвращает статистику по постам за указанный период
	GetPostPlatformStatsByDateRange(startDate, endDate time.Time, platform string) ([]*entity.PostPlatformStats, error)
	// GetCommentsCountByPeriod возвращает количество комментариев к посту за указанный период
	GetCommentsCountByPeriod(postUnionID int, startDate, endDate time.Time) (int, error)
	// CommentsCount возвращает количество комментариев к посту
	CommentsCount(postUnionID int) (int, error)
	// SavePostPlatformStats сохраняет новую статистику поста
	SavePostPlatformStats(stats *entity.PostPlatformStats) error

	// GetUserKPI возвращает KPI по посту
	GetUserKPI(userID int, startDate, endDate time.Time) (*entity.UserKPI, error)
	// CompareUserKPI возвращает KPI по постам для нескольких пользователей
	CompareUserKPI(userIDs []int, startDate, endDate time.Time) (map[int]*entity.UserKPI, error)

	// Методы для работы с задачами обновления статистики
	// CreateStatsUpdateTask создает задачу на обновление статистики
	CreateStatsUpdateTask(postUnionID int, platform string) error
	// GetPendingStatsUpdateTasks возвращает задачи, готовые к выполнению
	GetPendingStatsUpdateTasks(workerID string, limit int) ([]*entity.StatsUpdateTask, error)
	// LockStatsUpdateTask блокирует задачу для выполнения
	LockStatsUpdateTask(taskID int, workerID string) error
	// UpdateStatsUpdateTask обновляет задачу после выполнения
	UpdateStatsUpdateTask(taskID int, nextUpdateAt time.Time, intervalMinutes int) error
	// UnlockStatsUpdateTask разблокирует задачу
	UnlockStatsUpdateTask(taskID int) error
	// DeleteStatsUpdateTask удаляет задачу (для постов старше 6 месяцев)
	DeleteStatsUpdateTask(taskID int) error
}

var (
	ErrPostPlatformStatsNotFound = errors.New("post platform stats not found")
)
