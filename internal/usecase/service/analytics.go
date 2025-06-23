package service

import (
	"errors"
	"fmt"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"slices"
	"time"

	"github.com/labstack/gommon/log"
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
		// Получаем статистику за указанный период
		stats, err := a.analyticsRepo.GetPostPlatformStatsByDateRange(request.Start, request.End, platform)
		if err != nil && !errors.Is(err, repo.ErrPostPlatformStatsNotFound) {
			return nil, fmt.Errorf("ошибка получения статистики: %w", err)
		}

		// Если нет статистики, пропускаем
		if errors.Is(err, repo.ErrPostPlatformStatsNotFound) {
			continue
		}

		// Группируем по постам и агрегируем данные
		postStatsMap := make(map[int]*entity.PostPlatformStats)

		for _, stat := range stats {
			if stat.TeamID != request.TeamID {
				// Пропускаем статистику, если она не для текущей команды
				continue
			}
			if existing, exists := postStatsMap[stat.PostUnionID]; !exists {
				postStatsMap[stat.PostUnionID] = &entity.PostPlatformStats{
					TeamID:      stat.TeamID,
					PostUnionID: stat.PostUnionID,
					Platform:    stat.Platform,
					Views:       stat.Views,
					Reactions:   stat.Reactions,
				}
			} else {
				// Берем максимальные значения за период
				if stat.Views > existing.Views {
					existing.Views = stat.Views
				}
				if stat.Reactions > existing.Reactions {
					existing.Reactions = stat.Reactions
				}
			}
		}

		// Обрабатываем агрегированные данные
		for _, periodStats := range postStatsMap {
			// Получаем количество комментариев за период
			comments, err := a.analyticsRepo.GetCommentsCountByPeriod(periodStats.PostUnionID, request.Start, request.End)
			if err != nil {
				return nil, fmt.Errorf("ошибка получения комментариев за период: %w", err)
			}
			periodStats.Comments = comments

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
			// Если статистики пока не существует, создаём задачу на обновление и возвращаем нулевые статистики
			err = a.analyticsRepo.CreateStatsUpdateTask(postUnion.ID, platform)
			if err != nil {
				log.Errorf("Error creating stats update task: %v", err)
			}
			stats = &entity.PostPlatformStats{
				TeamID:      postUnion.TeamID,
				PostUnionID: request.PostUnionID,
				Platform:    platform,
				RecordedAt:  time.Now(),
				Views:       0,
				Comments:    0,
				Reactions:   0,
			}
		} else if err != nil {
			return nil, fmt.Errorf("failed to get stats: %w", err)
		}
		if stats.TeamID == request.TeamID {
			allStats[i] = stats
		}
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
	// Получаем всех участников команды
	userIDs, err := a.teamRepo.GetTeamUsers(request.TeamID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения пользователей команды: %w", err)
	}

	// Получаем KPI для всех пользователей
	kpiMap, err := a.analyticsRepo.CompareUserKPI(userIDs, request.Start, request.End)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения KPI: %w", err)
	}

	// Преобразуем в массив
	kpiList := make([]entity.UserKPI, 0, len(kpiMap))
	for _, kpi := range kpiMap {
		kpiList = append(kpiList, *kpi)
	}

	return &entity.UsersKPIResponse{
		Users: kpiList,
	}, nil
}

func (a *Analytics) ProcessStatsUpdateTasks(workerID string) error {
	// Получаем задачи для обработки (до 10 за раз)
	tasks, err := a.analyticsRepo.GetPendingStatsUpdateTasks(workerID, 10)
	if err != nil {
		return fmt.Errorf("ошибка получения задач: %w", err)
	}

	for _, task := range tasks {
		if err := a.processStatsUpdateTask(task, workerID); err != nil {
			log.Errorf("Ошибка обработки задачи %d: %v", task.ID, err)
			// Разблокируем задачу в случае ошибки
			if unlockErr := a.analyticsRepo.UnlockStatsUpdateTask(task.ID); unlockErr != nil {
				log.Errorf("Ошибка разблокировки задачи %d: %v", task.ID, unlockErr)
			}
		}
	}

	return nil
}

func (a *Analytics) processStatsUpdateTask(task *entity.StatsUpdateTask, workerID string) error {
	// Блокируем задачу
	if err := a.analyticsRepo.LockStatsUpdateTask(task.ID, workerID); err != nil {
		return fmt.Errorf("не удалось заблокировать задачу: %w", err)
	}

	// Проверяем возраст поста
	postUnion, err := a.postRepo.GetPostUnion(task.PostUnionID)
	if err != nil {
		return fmt.Errorf("не удалось получить пост: %w", err)
	}

	postAge := time.Since(postUnion.CreatedAt)
	if postUnion.PubDate != nil {
		postAge = time.Since(*postUnion.PubDate)
	}

	// Если пост старше 6 месяцев, удаляем задачу
	if postAge > 6*30*24*time.Hour {
		return a.analyticsRepo.DeleteStatsUpdateTask(task.ID)
	}

	// Обновляем статистику в зависимости от платформы
	switch task.Platform {
	case "tg":
		err = a.telegramAnalytics.UpdateStat(task.PostUnionID)
	case "vk":
		err = a.vkontakteAnalytics.UpdateStat(task.PostUnionID)
	default:
		return fmt.Errorf("неизвестная платформа: %s", task.Platform)
	}

	if err != nil {
		return fmt.Errorf("ошибка обновления статистики: %w", err)
	}

	// Вычисляем следующий интервал обновления
	nextInterval := a.calculateNextInterval(task.UpdateIntervalMinutes, postAge)
	nextUpdateAt := time.Now().Add(time.Duration(nextInterval) * time.Minute)

	// Обновляем задачу
	return a.analyticsRepo.UpdateStatsUpdateTask(task.ID, nextUpdateAt, nextInterval)
}

func (a *Analytics) calculateNextInterval(currentInterval int, postAge time.Duration) int {
	// Начинаем с 10 минут, затем экспоненциально увеличиваем до суток
	hours := int(postAge.Hours())

	switch {
	case hours < 1: // Первый час - каждые 10 минут
		return 10
	case hours < 6: // Первые 6 часов - каждые 30 минут
		return 30
	case hours < 24: // Первые сутки - каждый час
		return 60
	case hours < 72: // Первые 3 дня - каждые 3 часа
		return 180
	case hours < 168: // Первая неделя - каждые 6 часов
		return 360
	case hours < 720: // Первый месяц - каждые 12 часов
		return 720
	default: // После месяца - раз в сутки
		return 1440
	}
}
