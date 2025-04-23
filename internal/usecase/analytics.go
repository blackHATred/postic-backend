package usecase

import "postic-backend/internal/entity"

type AnalyticsPlatform interface {
	// UpdateStat обновляет и возвращает статистику по посту по конкретной платформе
	UpdateStat(postUnionID int) (*entity.PlatformStats, error)
}

type Analytics interface {
	// UpdatePostStats обновляет статистику по посту
	UpdatePostStats(request *entity.UpdatePostStatsRequest) error
	// GetStats возвращает статистику по постам
	GetStats(request *entity.GetStatsRequest) (*entity.StatsResponse, error)
	// GetPostUnionStats возвращает статистику по посту
	GetPostUnionStats(request *entity.GetPostUnionStatsRequest) ([]*entity.PostPlatformStats, error)
}
