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
	postsMap := make(map[int]*entity.PostStats) // Карта для группировки статистики по postUnionID

	for _, platform := range platforms {
		// Получаем статистику на начало периода (ближайшая к request.Start)
		statsStart, err := a.analyticsRepo.GetPostPlatformStatsByNearestDate(request.Start, platform, false)
		if err != nil && !errors.Is(err, repo.ErrPostPlatformStatsNotFound) {
			return nil, fmt.Errorf("ошибка получения статистики на начало периода: %w", err)
		}

		// Получаем статистику на конец периода (ближайшая к request.End)
		statsEnd, err := a.analyticsRepo.GetPostPlatformStatsByNearestDate(request.End, platform, true)
		if err != nil && !errors.Is(err, repo.ErrPostPlatformStatsNotFound) {
			return nil, fmt.Errorf("ошибка получения статистики на конец периода: %w", err)
		}

		// Если нет статистики на начало или конец периода, пропускаем
		if errors.Is(err, repo.ErrPostPlatformStatsNotFound) {
			continue
		}

		// Группируем статистику по postUnionID и вычисляем разницу
		startStatsMap := make(map[int]*entity.PostPlatformStats)
		for _, stat := range statsStart {
			startStatsMap[stat.PostUnionID] = stat
		}

		for _, endStat := range statsEnd {
			// Создаем объект статистики для периода
			periodStats := &entity.PostPlatformStats{
				TeamID:      endStat.TeamID,
				PostUnionID: endStat.PostUnionID,
				Platform:    endStat.Platform,
				PeriodStart: request.Start,
				PeriodEnd:   &request.End,
				Views:       endStat.Views,
				Reactions:   endStat.Reactions,
				Comments:    endStat.Comments,
			}

			// Если есть статистика на начало периода, вычитаем её
			if startStat, exists := startStatsMap[endStat.PostUnionID]; exists {
				periodStats.Views = endStat.Views - startStat.Views
				periodStats.Reactions = endStat.Reactions - startStat.Reactions

				// Для комментариев нам нужно получить точное количество за период
				comments, err := a.analyticsRepo.GetCommentsCountByPeriod(endStat.PostUnionID, request.Start, request.End)
				if err != nil {
					return nil, fmt.Errorf("ошибка получения комментариев за период: %w", err)
				}
				periodStats.Comments = comments
			} else {
				// Если нет статистики на начало, считаем все комментарии до конца периода
				comments, err := a.analyticsRepo.GetCommentsCountByPeriod(endStat.PostUnionID, time.Time{}, request.End)
				if err != nil {
					return nil, fmt.Errorf("ошибка получения комментариев за период: %w", err)
				}
				periodStats.Comments = comments
			}

			// Обеспечиваем, чтобы значения не были отрицательными
			if periodStats.Views < 0 {
				periodStats.Views = 0
			}
			if periodStats.Reactions < 0 {
				periodStats.Reactions = 0
			}

			// Создаём объект с данными платформы
			platformStats := &entity.PlatformStats{
				Views:     periodStats.Views,
				Comments:  periodStats.Comments,
				Reactions: periodStats.Reactions,
			}

			// Проверяем, есть ли уже запись для этого поста
			postStats, exists := postsMap[periodStats.PostUnionID]
			if !exists {
				// Создаем новую запись, если её еще нет
				postStats = &entity.PostStats{
					PostUnionID: periodStats.PostUnionID,
				}
				postsMap[periodStats.PostUnionID] = postStats
			}

			// Добавляем статистику для соответствующей платформы
			switch periodStats.Platform {
			case "tg":
				postStats.Telegram = platformStats
			case "vk":
				postStats.Vkontakte = platformStats
			}
		}
	}

	// Преобразуем карту в массив для ответа
	posts := make([]*entity.PostStats, 0, len(postsMap))
	for _, postStats := range postsMap {
		posts = append(posts, postStats)
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
