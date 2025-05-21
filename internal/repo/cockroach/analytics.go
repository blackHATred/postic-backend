package cockroach

import (
	"database/sql"
	"errors"
	"fmt"
	sq "github.com/Masterminds/squirrel"
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
	query, args, err := sq.Select("id", "team_id", "post_union_id", "period_start", "period_end", "platform", "views", "reactions").
		From("post_platform_stats_history").
		Where(sq.Eq{"post_union_id": postUnionID}).
		Where(sq.Eq{"platform": platform}).
		OrderBy("period_start DESC").
		Limit(1).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("ошибка при формировании SQL-запроса: %w", err)
	}

	stats := &entity.PostPlatformStats{}
	err = a.db.Get(stats, query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repo.ErrPostPlatformStatsNotFound
		}
		return nil, fmt.Errorf("ошибка при получении статистики поста: %w", err)
	}

	// Получаем количество комментариев из отдельной таблицы
	comments, err := a.CommentsCount(postUnionID)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении количества комментариев: %w", err)
	}
	stats.Comments = comments

	return stats, nil
}

func (a *Analytics) GetPostPlatformStatsByPeriod(startDate, endDate time.Time, platform string) ([]*entity.PostPlatformStats, error) {
	query, args, err := sq.Select("id", "team_id", "post_union_id", "period_start", "period_end", "platform", "views", "reactions").
		From("post_platform_stats_history").
		Where(sq.Eq{"platform": platform}).
		Where(sq.Gt{"period_start": startDate}).
		Where(sq.Or{
			sq.Eq{"period_end": nil},
			sq.Lt{"period_end": endDate},
		}).
		OrderBy("period_start DESC").
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("ошибка при формировании SQL-запроса: %w", err)
	}

	var stats []*entity.PostPlatformStats
	err = a.db.Select(&stats, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении статистики по периоду: %w", err)
	}

	// Получаем количество комментариев для каждого поста
	for _, stat := range stats {
		comments, err := a.CommentsCount(stat.PostUnionID)
		if err != nil {
			return nil, fmt.Errorf("ошибка при получении количества комментариев для поста %d: %w", stat.PostUnionID, err)
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
		query, args, err := sq.Select("team_id").
			From("post_union").
			Where(sq.Eq{"id": postUnionID}).
			PlaceholderFormat(sq.Dollar).
			ToSql()

		if err != nil {
			return fmt.Errorf("ошибка при формировании SQL-запроса для получения team_id: %w", err)
		}

		var teamID int
		err = a.db.QueryRow(query, args...).Scan(&teamID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return repo.ErrPostUnionNotFound
			}
			return fmt.Errorf("не удалось получить team_id: %w", err)
		}

		// Создаем первую запись с нулевыми значениями
		insertQuery, insertArgs, err := sq.Insert("post_platform_stats_history").
			Columns("team_id", "post_union_id", "platform", "period_start", "period_end", "views", "reactions").
			Values(teamID, postUnionID, platform, sq.Expr("NOW()"), nil, 0, 0).
			PlaceholderFormat(sq.Dollar).
			ToSql()

		if err != nil {
			return fmt.Errorf("ошибка при формировании SQL-запроса для создания записи: %w", err)
		}

		_, err = a.db.Exec(insertQuery, insertArgs...)
		if err != nil {
			return fmt.Errorf("не удалось создать начальный период статистики: %w", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("ошибка при получении последней статистики: %w", err)
	}

	// Создаем новый период со значениями из последнего периода
	insertQuery, insertArgs, err := sq.Insert("post_platform_stats_history").
		Columns("team_id", "post_union_id", "platform", "period_start", "period_end", "views", "reactions").
		Values(lastStats.TeamID, postUnionID, platform, sq.Expr("NOW()"), nil, lastStats.Views, lastStats.Reactions).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return fmt.Errorf("ошибка при формировании SQL-запроса для создания нового периода: %w", err)
	}

	_, err = a.db.Exec(insertQuery, insertArgs...)
	if err != nil {
		return fmt.Errorf("не удалось создать новый период статистики: %w", err)
	}

	return nil
}

func (a *Analytics) EndPeriod(postUnionID int, platform string) error {
	query, args, err := sq.Update("post_platform_stats_history").
		Set("period_end", sq.Expr("NOW()")).
		Where(sq.Eq{"post_union_id": postUnionID}).
		Where(sq.Eq{"platform": platform}).
		Where(sq.Eq{"period_end": nil}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return fmt.Errorf("ошибка при формировании SQL-запроса для завершения периода: %w", err)
	}

	_, err = a.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("ошибка при завершении периода: %w", err)
	}

	return nil
}

func (a *Analytics) UpdateLastPlatformStats(stats *entity.PostPlatformStats, platform string) error {
	// Проверяем, есть ли вообще какие-то записи для этого поста
	existsQuery, existsArgs, err := sq.Select("1").
		From("post_platform_stats_history").
		Where(sq.Eq{"post_union_id": stats.PostUnionID}).
		Where(sq.Eq{"platform": platform}).
		Prefix("SELECT EXISTS(").
		Suffix(")").
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return fmt.Errorf("ошибка при формировании SQL-запроса для проверки существования: %w", err)
	}

	var exists bool
	err = a.db.QueryRow(existsQuery, existsArgs...).Scan(&exists)
	if err != nil {
		return fmt.Errorf("ошибка при проверке существования статистики: %w", err)
	}

	// Если записей нет, создаём новую
	if !exists {
		insertQuery, insertArgs, err := sq.Insert("post_platform_stats_history").
			Columns("team_id", "post_union_id", "platform", "period_start", "views", "reactions").
			Values(stats.TeamID, stats.PostUnionID, platform, sq.Expr("NOW()"), stats.Views, stats.Reactions).
			PlaceholderFormat(sq.Dollar).
			ToSql()

		if err != nil {
			return fmt.Errorf("ошибка при формировании SQL-запроса для создания статистики: %w", err)
		}

		_, err = a.db.Exec(insertQuery, insertArgs...)
		if err != nil {
			return fmt.Errorf("ошибка при создании начальной статистики: %w", err)
		}
		return nil
	}

	// Ищем активный период
	activeQuery, activeArgs, err := sq.Select("id").
		From("post_platform_stats_history").
		Where(sq.Eq{"post_union_id": stats.PostUnionID}).
		Where(sq.Eq{"platform": platform}).
		Where(sq.Eq{"period_end": nil}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return fmt.Errorf("ошибка при формировании SQL-запроса для поиска активного периода: %w", err)
	}

	var id int
	err = a.db.QueryRow(activeQuery, activeArgs...).Scan(&id)

	if errors.Is(err, sql.ErrNoRows) {
		// Нет активного периода - создаём новый на основе последней записи
		lastStatsQuery, lastStatsArgs, err := sq.Select("id", "team_id", "views", "reactions").
			From("post_platform_stats_history").
			Where(sq.Eq{"post_union_id": stats.PostUnionID}).
			Where(sq.Eq{"platform": platform}).
			OrderBy("period_start DESC").
			Limit(1).
			PlaceholderFormat(sq.Dollar).
			ToSql()

		if err != nil {
			return fmt.Errorf("ошибка при формировании SQL-запроса для получения последней статистики: %w", err)
		}

		var lastStats entity.PostPlatformStats
		err = a.db.QueryRow(lastStatsQuery, lastStatsArgs...).Scan(&lastStats.ID, &lastStats.TeamID, &lastStats.Views, &lastStats.Reactions)
		if err != nil {
			return fmt.Errorf("ошибка при получении последней статистики: %w", err)
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

		insertNewQuery, insertNewArgs, err := sq.Insert("post_platform_stats_history").
			Columns("team_id", "post_union_id", "platform", "period_start", "views", "reactions").
			Values(lastStats.TeamID, stats.PostUnionID, platform, sq.Expr("NOW()"), views, reactions).
			PlaceholderFormat(sq.Dollar).
			ToSql()

		if err != nil {
			return fmt.Errorf("ошибка при формировании SQL-запроса для создания нового периода: %w", err)
		}

		_, err = a.db.Exec(insertNewQuery, insertNewArgs...)
		if err != nil {
			return fmt.Errorf("ошибка при создании нового периода статистики: %w", err)
		}
		return nil
	}

	if err != nil {
		return fmt.Errorf("ошибка при поиске активного периода: %w", err)
	}

	// Обновляем существующую запись
	updateQuery, updateArgs, err := sq.Update("post_platform_stats_history").
		Set("views", sq.Expr("CASE WHEN ? > 0 THEN ? ELSE views END", stats.Views, stats.Views)).
		Set("reactions", sq.Expr("CASE WHEN ? > 0 THEN ? ELSE reactions END", stats.Reactions, stats.Reactions)).
		Where(sq.Eq{"id": id}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return fmt.Errorf("ошибка при формировании SQL-запроса для обновления статистики: %w", err)
	}

	_, err = a.db.Exec(updateQuery, updateArgs...)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении статистики: %w", err)
	}

	return nil
}

func (a *Analytics) CommentsCount(postUnionID int) (int, error) {
	query, args, err := sq.Select("COUNT(*)").
		From("post_comment").
		Where(sq.Eq{"post_union_id": postUnionID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return 0, fmt.Errorf("ошибка при формировании SQL-запроса для подсчета комментариев: %w", err)
	}

	var count int
	err = a.db.Get(&count, query, args...)
	if err != nil {
		return 0, fmt.Errorf("ошибка при подсчете комментариев: %w", err)
	}

	return count, nil
}
