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

func (a *Analytics) GetPostPlatformStatsByPeriod(startDate, endDate time.Time, platform string) ([]*entity.PostPlatformStats, error) {
	query := `
			SELECT id, team_id, post_union_id, period_start, period_end, platform, views, reactions
			FROM post_platform_stats_history
			WHERE platform = $1 AND period_start > $2 AND (period_end IS NULL OR period_end < $3)
			ORDER BY period_start DESC
		`

	var stats []*entity.PostPlatformStats
	err := a.db.Select(&stats, query, platform, startDate, endDate)
	if err != nil {
		return nil, err
	}

	// Получаем количество комментариев для каждого поста
	for _, stat := range stats {
		comments, err := a.CommentsCount(stat.PostUnionID)
		if err != nil {
			return nil, err
		}
		stat.Comments = comments
	}

	return stats, nil
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
