package cockroach

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"time"
)

type Analytics struct {
	db *sqlx.DB
}

func NewAnalytics(db *sqlx.DB) repo.Analytics {
	return &Analytics{
		db: db,
	}
}

func (a *Analytics) GetPostPlatformStatsByPostUnionID(postUnionID int, platform string) (*entity.PostPlatformStats, error) {
	query := `
		SELECT id, team_id, post_union_id, period_start, period_end, platform, views, reactions
		FROM post_platform_stats_history
		WHERE post_union_id = $1 AND platform = $2
		ORDER BY period_start DESC
		LIMIT 1
	`

	stats := &entity.PostPlatformStats{}
	err := a.db.Get(stats, query, postUnionID, platform)
	if err != nil {
		return nil, repo.ErrPostPlatformStatsNotFound
	}

	// Получаем количество комментариев из отдельной таблицы
	comments, err := a.CommentsCount(postUnionID)
	if err != nil {
		return nil, err
	}
	stats.Comments = comments

	return stats, nil
}

func (a *Analytics) countCommentsByPeriod(postUnionID int, startDate, endDate time.Time) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM post_comment
		WHERE post_union_id = $1 AND created_at >= $2 AND created_at <= $3
	`

	var count int
	err := a.db.Get(&count, query, postUnionID, startDate, endDate)
	if err != nil {
		return 0, fmt.Errorf("failed to count comments by period: %w", err)
	}

	return count, nil
}

func (a *Analytics) GetPostPlatformStatsByPeriod(startDate, endDate time.Time, platform string) (*entity.PostPlatformStats, error) {
	// Получаем статистику, которая заканчивается ближе всего к endDate
	queryEnd := `
		SELECT id, team_id, post_union_id, platform, views, reactions
		FROM post_platform_stats_history
		WHERE platform = $1 AND period_start <= $2 AND (period_end IS NULL OR period_end >= $2)
		ORDER BY period_start DESC
		LIMIT 1
	`
	endStats := &entity.PostPlatformStats{}
	err := a.db.Get(endStats, queryEnd, platform, endDate)
	if errors.Is(err, sql.ErrNoRows) {
		// возвращаем нулевые статистики, если ничего не найдено
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get end stats: %w", err)
	}

	// Получаем статистику, которая начинается ближе всего к startDate
	queryStart := `
		SELECT id, team_id, post_union_id, platform, views, reactions
		FROM post_platform_stats_history
		WHERE platform = $1 AND period_start >= $2
		ORDER BY period_start ASC
		LIMIT 1
	`
	startStats := &entity.PostPlatformStats{}
	err = a.db.Get(startStats, queryStart, platform, startDate)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to get start stats: %w", err)
	}
	//log.Infof("start stats: views=%v reactions=%v", startStats.Views, startStats.Reactions)
	//log.Infof("end stats: views=%v reactions=%v", endStats.Views, endStats.Reactions)

	// Вычитаем начальную статистику из конечной
	resultStats := &entity.PostPlatformStats{
		TeamID:      endStats.TeamID,
		PostUnionID: endStats.PostUnionID,
		Platform:    endStats.Platform,
		Views:       endStats.Views - startStats.Views,
		Reactions:   endStats.Reactions - startStats.Reactions,
	}

	// Получаем количество комментариев для итоговой статистики
	comments, err := a.countCommentsByPeriod(resultStats.PostUnionID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments count: %w", err)
	}
	resultStats.Comments = comments

	return resultStats, nil
}

func (a *Analytics) CreateNewPeriod(postUnionID int, platform string) error {
	// Получаем последнюю статистику
	lastStats, err := a.GetPostPlatformStatsByPostUnionID(postUnionID, platform)

	if errors.Is(err, repo.ErrPostPlatformStatsNotFound) {
		// Если статистики еще нет, то необходимо получить team_id
		var teamID int
		err := a.db.QueryRow("SELECT team_id FROM post_union WHERE id = $1", postUnionID).Scan(&teamID)
		if err != nil {
			return fmt.Errorf("не удалось получить team_id: %w", err)
		}

		// Создаем первую запись с нулевыми значениями
		query := `
            INSERT INTO post_platform_stats_history (
                team_id, post_union_id, platform, period_start, period_end, views, reactions
            ) VALUES (
                $1, $2, $3, NOW(), NULL, 0, 0
            )`

		_, err = a.db.Exec(query, teamID, postUnionID, platform)
		if err != nil {
			return fmt.Errorf("не удалось создать начальный период статистики: %w", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("ошибка при получении последней статистики: %w", err)
	}

	// Создаем новый период со значениями из последнего периода
	query := `
        INSERT INTO post_platform_stats_history (
            team_id, post_union_id, platform, period_start, period_end, views, reactions
        ) VALUES (
            $1, $2, $3, NOW(), NULL, $4, $5
        )`

	_, err = a.db.Exec(query, lastStats.TeamID, postUnionID, platform, lastStats.Views, lastStats.Reactions)
	if err != nil {
		return fmt.Errorf("не удалось создать новый период статистики: %w", err)
	}

	return nil
}

func (a *Analytics) EndPeriod(postUnionID int, platform string) error {
	query := `
		UPDATE post_platform_stats_history
		SET period_end = NOW()
		WHERE post_union_id = $1 AND platform = $2 AND period_end IS NULL
	`
	_, err := a.db.Exec(query, postUnionID, platform)
	return err
}

func (a *Analytics) UpdateLastPlatformStats(stats *entity.PostPlatformStats, platform string) error {
	// Проверяем, есть ли вообще какие-то записи для этого поста
	var exists bool
	err := a.db.QueryRow(`
        SELECT EXISTS(SELECT 1 FROM post_platform_stats_history 
        WHERE post_union_id = $1 AND platform = $2)
    `, stats.PostUnionID, platform).Scan(&exists)

	if err != nil {
		return fmt.Errorf("failed to check stats existence: %w", err)
	}

	// Если записей нет, создаём новую
	if !exists {
		_, err := a.db.Exec(`
            INSERT INTO post_platform_stats_history 
            (team_id, post_union_id, platform, period_start, views, reactions)
            VALUES ($1, $2, $3, NOW(), $4, $5)
        `, stats.TeamID, stats.PostUnionID, platform, stats.Views, stats.Reactions)
		return err
	}

	// Ищем активный период
	var id int
	err = a.db.QueryRow(`
        SELECT id FROM post_platform_stats_history
        WHERE post_union_id = $1 
          AND platform = $2
          AND period_end IS NULL
    `, stats.PostUnionID, platform).Scan(&id)

	if errors.Is(err, sql.ErrNoRows) {
		// Нет активного периода - создаём новый на основе последней записи
		var lastStats entity.PostPlatformStats
		err = a.db.QueryRow(`
            SELECT id, team_id, views, reactions FROM post_platform_stats_history
            WHERE post_union_id = $1 AND platform = $2
            ORDER BY period_start DESC LIMIT 1
        `, stats.PostUnionID, platform).Scan(&lastStats.ID, &lastStats.TeamID, &lastStats.Views, &lastStats.Reactions)

		if err != nil {
			return fmt.Errorf("failed to get last stats: %w", err)
		}

		// Обновляем значения, если новые значения больше нуля
		views := lastStats.Views
		if stats.Views > 0 {
			views = stats.Views
		}

		reactions := lastStats.Reactions
		if stats.Reactions > 0 {
			reactions = stats.Reactions
		}

		_, err = a.db.Exec(`
            INSERT INTO post_platform_stats_history 
            (team_id, post_union_id, platform, period_start, views, reactions)
            VALUES ($1, $2, $3, NOW(), $4, $5)
        `, lastStats.TeamID, stats.PostUnionID, platform, views, reactions)
		return err
	}

	// Обновляем существующую запись
	_, err = a.db.Exec(`
        UPDATE post_platform_stats_history
        SET views = CASE WHEN $1 > 0 THEN $1 ELSE views END,
            reactions = CASE WHEN $2 > 0 THEN $2 ELSE reactions END
        WHERE id = $3
    `, stats.Views, stats.Reactions, id)

	return err
}

func (a *Analytics) CommentsCount(postUnionID int) (int, error) {
	query := `
        SELECT COUNT(*) 
        FROM post_comment 
        WHERE post_union_id = $1
    `

	var count int
	err := a.db.Get(&count, query, postUnionID)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (a *Analytics) GetUserKPI(userID int, startDate, endDate time.Time) (float64, error) {
	// Получаем все посты пользователя за указанный период
	queryPosts := `
		SELECT id
		FROM post_union
		WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
	`
	var postIDs []int
	err := a.db.Select(&postIDs, queryPosts, userID, startDate, endDate)
	if err != nil {
		return 0, fmt.Errorf("failed to get posts for user: %w", err)
	}

	if len(postIDs) == 0 {
		return 0, nil // Нет постов за указанный период
	}

	// Инициализируем метрики
	var totalViews, totalReactions, totalComments int

	// Считаем статистику по каждому посту
	for _, postID := range postIDs {
		// Получаем просмотры и реакции из post_platform_stats_history
		queryStats := `
			SELECT COALESCE(SUM(views), 0) AS views, COALESCE(SUM(reactions), 0) AS reactions
			FROM post_platform_stats_history
			WHERE post_union_id = $1 AND period_start >= $2 AND (period_end <= $3 OR period_end IS NULL)
		`
		var views, reactions int
		err := a.db.QueryRow(queryStats, postID, startDate, endDate).Scan(&views, &reactions)
		if err != nil {
			return 0, fmt.Errorf("failed to get stats for post %d: %w", postID, err)
		}

		// Получаем количество комментариев из post_comment
		queryComments := `
			SELECT COUNT(*)
			FROM post_comment
			WHERE post_union_id = $1 AND created_at >= $2 AND created_at <= $3
		`
		var comments int
		err = a.db.QueryRow(queryComments, postID, startDate, endDate).Scan(&comments)
		if err != nil {
			return 0, fmt.Errorf("failed to get comments for post %d: %w", postID, err)
		}

		// Суммируем метрики
		totalViews += views
		totalReactions += reactions
		totalComments += comments
	}

	// Рассчитываем KPI
	const (
		weightViews     = 0.1
		weightReactions = 0.3
		weightComments  = 1.2
	)
	kpi := float64(totalViews)*weightViews + float64(totalReactions)*weightReactions + float64(totalComments)*weightComments

	return kpi, nil
}

func (a *Analytics) CompareUserKPI(userIDs []int, startDate, endDate time.Time) (map[int]float64, error) {
	kpiResults := make(map[int]float64)

	for _, userID := range userIDs {
		kpi, err := a.GetUserKPI(userID, startDate, endDate)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate KPI for user %d: %w", userID, err)
		}
		kpiResults[userID] = kpi
	}

	return kpiResults, nil
}
