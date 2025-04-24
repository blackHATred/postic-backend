package telegram

import (
	"errors"
	"fmt"
	"github.com/go-telegram/bot"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"time"
)

type Analytics struct {
	bot           *bot.Bot
	teamRepo      repo.Team
	postRepo      repo.Post
	analyticsRepo repo.Analytics
}

func NewTelegramAnalytics(
	token string,
	teamRepo repo.Team,
	postRepo repo.Post,
	analyticsRepo repo.Analytics,
) (usecase.AnalyticsPlatform, error) {
	telegramBot, err := bot.New(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram telegramBot: %w", err)
	}
	return &Analytics{
		bot:           telegramBot,
		teamRepo:      teamRepo,
		postRepo:      postRepo,
		analyticsRepo: analyticsRepo,
	}, nil
}

func (a *Analytics) UpdateStat(postUnionID int) (*entity.PlatformStats, error) {
	// Получаем пост по ID
	post, err := a.postRepo.GetPostUnion(postUnionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get post by ID: %w", err)
	}

	// Получаем статистику по посту
	stats, err := a.analyticsRepo.GetPostPlatformStatsByPostUnionID(postUnionID, "tg")
	switch {
	case errors.Is(err, repo.ErrPostPlatformStatsNotFound):
		// не нашли, в таком случае просто создаем новую статистику и возвращаем её
		stats = &entity.PostPlatformStats{
			PostUnionID: postUnionID,
			Platform:    "tg",
			TeamID:      post.TeamID,
			Views:       0,
			Comments:    0,
			Reactions:   0,
			LastUpdate:  time.Now(),
		}
		err = a.analyticsRepo.AddPostPlatformStats(stats)
		if err != nil {
			return nil, fmt.Errorf("failed to add post platform stats: %w", err)
		}
	case err != nil:
		return nil, fmt.Errorf("failed to get post platform stats: %w", err)
	}

	// Получаем количество комментариев к посту
	commentsCount, err := a.analyticsRepo.CommentsCount(postUnionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments count: %w", err)
	}
	stats.Comments = commentsCount

	// Получаем ориентировочное количество просмотров
	start := post.CreatedAt
	if post.PubDate != nil {
		start = *post.PubDate
	}
	views := EstimateViews(stats.Reactions, time.Since(start).Hours())
	stats.Views = views

	// Обновляем статистику
	err = a.analyticsRepo.EditPostPlatformStats(stats)
	if err != nil {
		return nil, fmt.Errorf("failed to edit post platform stats: %w", err)
	}

	return &entity.PlatformStats{
		Views:     stats.Views,
		Comments:  stats.Comments,
		Reactions: stats.Reactions,
	}, nil
}

func EstimateViews(reactions int, hoursPassed float64) int {
	if reactions == 0 {
		return 0
	}

	// Примерный CTR, характерный для большинства каналов — от 3% до 5%
	ctr := 0.04 // например, 4%

	// Временной множитель
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

	estimatedViews := (float64(reactions) / ctr) * timeFactor
	return int(estimatedViews)
}
