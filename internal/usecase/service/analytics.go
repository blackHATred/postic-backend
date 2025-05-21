package service

import (
	"errors"
	"fmt"
	"github.com/labstack/gommon/log"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"slices"
	"time"
)

type Analytics struct {
	analyticsRepo repo.Analytics
	teamRepo      repo.Team
	postRepo      repo.Post

	telegramAnalytics  usecase.AnalyticsPlatform
	vkontakteAnalytics usecase.AnalyticsPlatform
}

func NewAnalytics(
	analyticsRepo repo.Analytics,
	teamRepo repo.Team,
	postRepo repo.Post,
	telegramAnalytics usecase.AnalyticsPlatform,
	vkontakteAnalytics usecase.AnalyticsPlatform,
) usecase.Analytics {
	return &Analytics{
		analyticsRepo:      analyticsRepo,
		teamRepo:           teamRepo,
		postRepo:           postRepo,
		telegramAnalytics:  telegramAnalytics,
		vkontakteAnalytics: vkontakteAnalytics,
	}
}

// beginNewPeriod создает новый период для аналитики, если необходимо
func (a *Analytics) beginNewPeriod(postUnionID int, platform string) {
	// Получаем дату последнего обновления аналитики
	stats, err := a.analyticsRepo.GetPostPlatformStatsByPostUnionID(postUnionID, platform)
	if errors.Is(err, repo.ErrPostPlatformStatsNotFound) {
		// Если статистики пока не существует, то создаём новый период
		err = a.analyticsRepo.CreateNewPeriod(postUnionID, platform)
		if err != nil {
			log.Errorf("Error creating new period: %v", err)
			return
		}
		return
	}
	if err != nil {
		log.Errorf("Error getting last update date: %v", err)
		return
	}

	currentDate := time.Now()
	// Если время последнего отсчёта больше 5 минут назад, создаем новый период
	if currentDate.Sub(stats.PeriodStart) > 5*time.Minute {
		err = a.analyticsRepo.EndPeriod(postUnionID, platform)
		if err != nil {
			log.Errorf("Error ending period: %v", err)
			return
		}
		err = a.analyticsRepo.CreateNewPeriod(postUnionID, platform)
		if err != nil {
			log.Errorf("Error creating new period: %v", err)
			return
		}
		// Обновляем статистику по платформе
		switch platform {
		case "tg":
			err = a.telegramAnalytics.UpdateStat(postUnionID)
		case "vk":
			err = a.vkontakteAnalytics.UpdateStat(postUnionID)
		}
		if err != nil {
			log.Errorf("Error updating stats: %v", err)
			return
		}
	}
}

func (a *Analytics) GetStats(request *entity.GetStatsRequest) (*entity.StatsResponse, error) {
	// Проверяем права пользователя
	roles, err := a.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(roles, repo.AdminRole) && !slices.Contains(roles, repo.AnalyticsRole) {
		return nil, usecase.ErrUserForbidden
	}

	platforms := []string{"tg", "vk"}
	allStats := make([]*entity.PostPlatformStats, 0)
	for _, platform := range platforms {
		stats, err := a.analyticsRepo.GetPostPlatformStatsByPeriod(request.Start, request.End, platform)
		if err != nil {
			return nil, err
		}
		if stats != nil {
			allStats = append(allStats, stats)
		}
	}

	// составляем ответ
	posts := make([]*entity.PostStats, len(allStats))
	for i, postPlatformStats := range allStats {
		go a.beginNewPeriod(postPlatformStats.PostUnionID, postPlatformStats.Platform)
		platformStats := &entity.PlatformStats{
			Views:     postPlatformStats.Views,
			Comments:  postPlatformStats.Comments,
			Reactions: postPlatformStats.Reactions,
		}
		posts[i] = &entity.PostStats{
			PostUnionID: postPlatformStats.PostUnionID,
		}
		switch postPlatformStats.Platform {
		case "tg":
			posts[i].Telegram = platformStats
		case "vk":
			posts[i].Vkontakte = platformStats
		}
	}
	return &entity.StatsResponse{
		Posts: posts,
	}, nil
}

func (a *Analytics) GetPostUnionStats(request *entity.GetPostUnionStatsRequest) ([]*entity.PostPlatformStats, error) {
	// Проверяем права пользователя
	roles, err := a.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(roles, repo.AdminRole) && !slices.Contains(roles, repo.AnalyticsRole) {
		return nil, usecase.ErrUserForbidden
	}

	postUnion, err := a.postRepo.GetPostUnion(request.PostUnionID)
	if err != nil {
		return nil, err
	}

	allStats := make([]*entity.PostPlatformStats, len(postUnion.Platforms))

	for i, platform := range postUnion.Platforms {
		stats, err := a.analyticsRepo.GetPostPlatformStatsByPostUnionID(request.PostUnionID, platform)
		if errors.Is(err, repo.ErrPostPlatformStatsNotFound) {
			// Если статистики пока не существует, то создаём новый период и возвращаем нулевые статистики
			a.beginNewPeriod(postUnion.ID, platform)
			stats = &entity.PostPlatformStats{
				TeamID:      postUnion.TeamID,
				PostUnionID: request.PostUnionID,
				Platform:    platform,
				PeriodStart: time.Now(),
				Views:       0,
				Comments:    0,
				Reactions:   0,
			}
			return allStats, nil
		} else if err != nil {
			return nil, fmt.Errorf("failed to get stats: %w", err)
		}
		allStats[i] = stats
	}

	return allStats, nil
}

func (a *Analytics) GetUsersKPI(request *entity.GetUsersKPIRequest) (*entity.UsersKPIResponse, error) {
	// Проверяем права пользователя
	roles, err := a.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(roles, repo.AdminRole) && !slices.Contains(roles, repo.AnalyticsRole) {
		return nil, usecase.ErrUserForbidden
	}

	userIDs, err := a.teamRepo.GetTeamUsers(request.TeamID)
	if err != nil {
		return nil, err
	}
	kpi := make([]entity.UserKPI, len(userIDs))

	for i, userID := range userIDs {
		userKPI, err := a.analyticsRepo.GetUserKPI(userID, request.Start, request.End)
		if errors.Is(err, repo.ErrPostPlatformStatsNotFound) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("failed to get kpi: %w", err)
		}
		kpi[i] = entity.UserKPI{
			UserID:    userID,
			KPI:       userKPI.KPI,
			Views:     userKPI.Views,
			Comments:  userKPI.Comments,
			Reactions: userKPI.Reactions,
		}
	}

	return &entity.UsersKPIResponse{
		Users: kpi,
	}, nil
}
