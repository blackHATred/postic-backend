package service

import (
	"errors"
	"fmt"
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

func (a *Analytics) UpdatePostStats(request *entity.UpdatePostStatsRequest) error {
	// Проверяем права пользователя
	roles, err := a.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return err
	}
	if !slices.Contains(roles, repo.AdminRole) && !slices.Contains(roles, repo.AnalyticsRole) {
		return usecase.ErrUserForbidden
	}
	postUnion, err := a.postRepo.GetPostUnion(request.PostUnionID)
	if err != nil {
		return err
	}
	// Проверяем, когда было последнее обновление по каждой из платформ. Если прошло меньше 5 минут, то не обновляем
	for _, platform := range postUnion.Platforms {
		stats, err := a.analyticsRepo.GetPostPlatformStatsByPostUnionID(request.PostUnionID, platform)
		switch {
		case errors.Is(err, repo.ErrPostPlatformStatsNotFound):
			// Если статистики нет, то добавляем новую
			stats = &entity.PostPlatformStats{
				TeamID:      request.TeamID,
				PostUnionID: request.PostUnionID,
				Platform:    platform,
				Views:       0,
				Reactions:   0,
				Comments:    0,
				LastUpdate:  time.Now(),
			}
			err = a.analyticsRepo.AddPostPlatformStats(stats)
			if err != nil {
				return err
			}
		case err != nil:
			return err
		}
		if stats.LastUpdate.Add(5 * time.Minute).After(time.Now()) {
			continue
		}
		// Обновляем статистику по платформе
		switch platform {
		case "tg":
			// Обновляем статистику по платформе Telegram
			_, err := a.telegramAnalytics.UpdateStat(request.PostUnionID)
			if err != nil {
				return err
			}
		case "vk":
			// Обновляем статистику по платформе ВКонтакте
			_, err := a.vkontakteAnalytics.UpdateStat(request.PostUnionID)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return nil
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

	tgStats, err := a.analyticsRepo.GetPostPlatformStatsByPeriod(request.Start, request.End, "tg")
	if err != nil {
		return nil, err
	}
	vkStats, err := a.analyticsRepo.GetPostPlatformStatsByPeriod(request.Start, request.End, "vk")
	if err != nil {
		return nil, err
	}

	// составляем ответ
	posts := make([]*entity.PostStats, 0)
	for _, post := range tgStats {
		posts = append(posts, &entity.PostStats{
			PostUnionID: post.PostUnionID,
			Telegram: &entity.PlatformStats{
				Views:     post.Views,
				Comments:  post.Comments,
				Reactions: post.Reactions,
			},
		})
	}
	for _, post := range vkStats {
		posts = append(posts, &entity.PostStats{
			PostUnionID: post.PostUnionID,
			Vkontakte: &entity.PlatformStats{
				Views:     post.Views,
				Comments:  post.Comments,
				Reactions: post.Reactions,
			},
		})
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

	var allStats []*entity.PostPlatformStats

	for _, platform := range postUnion.Platforms {
		switch platform {
		case "tg":
			tgStats, err := a.analyticsRepo.GetPostPlatformStatsByPostUnionID(request.PostUnionID, "tg")
			if errors.Is(err, repo.ErrPostPlatformStatsNotFound) {
				// If no stats exist, create new ones
				tgStats = &entity.PostPlatformStats{
					TeamID:      request.TeamID,
					PostUnionID: request.PostUnionID,
					Platform:    "tg",
					Views:       0,
					Reactions:   0,
					Comments:    0,
					LastUpdate:  time.Now(),
				}
			} else if err != nil {
				return nil, fmt.Errorf("failed to get TG stats: %w", err)
			}
			allStats = append(allStats, tgStats)

		case "vk":
			vkStats, err := a.analyticsRepo.GetPostPlatformStatsByPostUnionID(request.PostUnionID, "vk")
			if errors.Is(err, repo.ErrPostPlatformStatsNotFound) {
				// If no stats exist, create new ones
				vkStats = &entity.PostPlatformStats{
					TeamID:      request.TeamID,
					PostUnionID: request.PostUnionID,
					Platform:    "vk",
					Views:       0,
					Reactions:   0,
					Comments:    0,
					LastUpdate:  time.Now(),
				}
			} else if err != nil {
				return nil, fmt.Errorf("failed to get VK stats: %w", err)
			}
			allStats = append(allStats, vkStats)
		}
	}
	// todo другие платформы

	return allStats, nil
}
