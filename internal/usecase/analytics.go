package usecase

import "postic-backend/internal/entity"

type AnalyticsPlatform interface {
	// UpdateStat обновляет и возвращает статистику по посту по конкретной платформе
	UpdateStat(postUnionID int) error
}

type Analytics interface {
	// GetStats возвращает статистику по постам
	GetStats(request *entity.GetStatsRequest) (*entity.StatsResponse, error)
	// GetPostUnionStats возвращает статистику по посту
	GetPostUnionStats(request *entity.GetPostUnionStatsRequest) ([]*entity.PostPlatformStats, error)
	// GetUsersKPI возвращает KPI по постам для нескольких пользователей
	GetUsersKPI(request *entity.GetUsersKPIRequest) (*entity.UsersKPIResponse, error)
}
