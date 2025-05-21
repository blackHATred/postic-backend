package telegram

import (
	"fmt"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"time"
)

type Analytics struct {
	teamRepo      repo.Team
	postRepo      repo.Post
	analyticsRepo repo.Analytics
}

func NewTelegramAnalytics(
	teamRepo repo.Team,
	postRepo repo.Post,
	analyticsRepo repo.Analytics,
) usecase.AnalyticsPlatform {
	return &Analytics{
		teamRepo:      teamRepo,
		postRepo:      postRepo,
		analyticsRepo: analyticsRepo,
	}
}

func (a *Analytics) UpdateStat(postUnionID int) error {
	// Получаем пост по ID
	post, err := a.postRepo.GetPostUnion(postUnionID)
	if err != nil {
		return fmt.Errorf("failed to get post by ID: %w", err)
	}

	stats := &entity.PostPlatformStats{
		TeamID:      post.TeamID,
		PostUnionID: postUnionID,
		Platform:    "tg",
	}

	// Получаем количество комментариев к посту
	commentsCount, err := a.analyticsRepo.CommentsCount(postUnionID)
	if err != nil {
		return fmt.Errorf("failed to get comments count: %w", err)
	}
	stats.Comments = commentsCount

	// Получаем ориентировочное количество просмотров
	start := post.CreatedAt
	if post.PubDate != nil {
		start = *post.PubDate
	}
	views := EstimateViews(stats.Reactions, stats.Comments, time.Since(start).Hours())
	stats.Views = views

	// Обновляем статистику
	err = a.analyticsRepo.UpdateLastPlatformStats(stats, "tg")
	if err != nil {
		return fmt.Errorf("failed to edit post platform stats: %w", err)
	}

	return nil
}

func EstimateViews(reactions int, comments int, hoursPassed float64) int {
	// Общий CTR для расчёта, по умолчанию 4%
	ctr := 0.04

	// Коэффициент времени
	timeFactor := 1.0
	switch {
	case hoursPassed < 1:
		timeFactor = 0.3
	case hoursPassed < 3:
		timeFactor = 0.6
	case hoursPassed < 12:
		timeFactor = 0.85
	default:
		timeFactor = 1.0
	}

	// Если комментариев нет, используем старую формулу
	if comments == 0 {
		return int((float64(reactions) / ctr) * timeFactor)
	}

	// Если комментарии есть, учитываем их при расчёте
	// Можно варьировать веса для реакций и комментариев
	reactionsWeight := float64(reactions) * 0.7
	commentsWeight := float64(comments) * 0.3
	estimatedViews := (reactionsWeight + commentsWeight) / ctr * timeFactor

	return int(estimatedViews)
}
