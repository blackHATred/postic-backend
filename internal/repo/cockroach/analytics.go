package cockroach

import (
	"database/sql"
	"errors"
	"fmt"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
)

type Analytics struct {
	db *sqlx.DB
}

func NewAnalytics(db *sqlx.DB) repo.Analytics {
	return &Analytics{db: db}
}

func (a *Analytics) GetPostPlatformStatsByPostUnionID(postUnionID int, platform string) (*entity.PostPlatformStats, error) {
	query, args, err := sq.Select("id", "team_id", "post_union_id", "recorded_at", "platform", "views", "reactions").
		From("post_platform_stats_history").
		Where(sq.Eq{"post_union_id": postUnionID}).
		Where(sq.Eq{"platform": platform}).
		OrderBy("recorded_at DESC").
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

func (a *Analytics) GetPostPlatformStatsByDateRange(startDate, endDate time.Time, platform string) ([]*entity.PostPlatformStats, error) {
	query, args, err := sq.Select("id", "team_id", "post_union_id", "recorded_at", "platform", "views", "reactions").
		From("post_platform_stats_history").
		Where(sq.Eq{"platform": platform}).
		Where(sq.GtOrEq{"recorded_at": startDate}).
		Where(sq.LtOrEq{"recorded_at": endDate}).
		OrderBy("post_union_id", "recorded_at DESC").
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("ошибка при формировании SQL-запроса: %w", err)
	}

	var allStats []*entity.PostPlatformStats
	err = a.db.Select(&allStats, query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repo.ErrPostPlatformStatsNotFound
		}
		return nil, fmt.Errorf("ошибка при получении статистики: %w", err)
	}

	if len(allStats) == 0 {
		return nil, repo.ErrPostPlatformStatsNotFound
	}

	// Получаем количество комментариев для каждой статистики
	for _, stats := range allStats {
		comments, err := a.CommentsCount(stats.PostUnionID)
		if err != nil {
			return nil, fmt.Errorf("ошибка при получении количества комментариев для поста %d: %w", stats.PostUnionID, err)
		}
		stats.Comments = comments
	}

	return allStats, nil
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

func (a *Analytics) SavePostPlatformStats(stats *entity.PostPlatformStats) error {
	query, args, err := sq.Insert("post_platform_stats_history").
		Columns("team_id", "post_union_id", "platform", "recorded_at", "views", "reactions").
		Values(stats.TeamID, stats.PostUnionID, stats.Platform, stats.RecordedAt, stats.Views, stats.Reactions).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return fmt.Errorf("ошибка при формировании SQL-запроса для сохранения статистики: %w", err)
	}

	_, err = a.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("ошибка при сохранении статистики: %w", err)
	}

	return nil
}

func (a *Analytics) CreateStatsUpdateTask(postUnionID int, platform string) error {
	// Проверяем, не существует ли уже такая задача
	checkQuery, checkArgs, err := sq.Select("id").
		From("stats_update_tasks").
		Where(sq.Eq{"post_union_id": postUnionID}).
		Where(sq.Eq{"platform": platform}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return fmt.Errorf("ошибка при формировании SQL-запроса для проверки существования задачи: %w", err)
	}

	var existingID int
	err = a.db.Get(&existingID, checkQuery, checkArgs...)
	if err == nil {
		// Задача уже существует
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("ошибка при проверке существования задачи: %w", err)
	}

	// Создаем новую задачу
	nextUpdate := time.Now().Add(10 * time.Minute) // Первое обновление через 10 минут
	query, args, err := sq.Insert("stats_update_tasks").
		Columns("post_union_id", "platform", "next_update_at", "update_interval_minutes").
		Values(postUnionID, platform, nextUpdate, 10).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return fmt.Errorf("ошибка при формировании SQL-запроса для создания задачи: %w", err)
	}

	_, err = a.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("ошибка при создании задачи: %w", err)
	}

	return nil
}

func (a *Analytics) GetPendingStatsUpdateTasks(workerID string, limit int) ([]*entity.StatsUpdateTask, error) {
	// Сначала разблокируем старые заблокированные задачи (более 5 минут)
	unlockQuery, unlockArgs, err := sq.Update("stats_update_tasks").
		Set("is_locked", false).
		Set("locked_at", nil).
		Set("locked_by", nil).
		Where(sq.Eq{"is_locked": true}).
		Where(sq.Lt{"locked_at": time.Now().Add(-5 * time.Minute)}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("ошибка при формировании SQL-запроса для разблокировки: %w", err)
	}

	_, err = a.db.Exec(unlockQuery, unlockArgs...)
	if err != nil {
		return nil, fmt.Errorf("ошибка при разблокировке старых задач: %w", err)
	}

	// Получаем задачи, готовые к выполнению
	query, args, err := sq.Select("id", "post_union_id", "platform", "next_update_at", "last_updated_at", "update_interval_minutes", "is_locked", "locked_at", "locked_by", "created_at").
		From("stats_update_tasks").
		Where(sq.Eq{"is_locked": false}).
		Where(sq.LtOrEq{"next_update_at": time.Now()}).
		OrderBy("next_update_at ASC").
		Limit(uint64(limit)).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("ошибка при формировании SQL-запроса для получения задач: %w", err)
	}

	var tasks []*entity.StatsUpdateTask
	err = a.db.Select(&tasks, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении задач: %w", err)
	}

	return tasks, nil
}

func (a *Analytics) LockStatsUpdateTask(taskID int, workerID string) error {
	query, args, err := sq.Update("stats_update_tasks").
		Set("is_locked", true).
		Set("locked_at", time.Now()).
		Set("locked_by", workerID).
		Where(sq.Eq{"id": taskID}).
		Where(sq.Eq{"is_locked": false}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return fmt.Errorf("ошибка при формировании SQL-запроса для блокировки задачи: %w", err)
	}

	result, err := a.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("ошибка при блокировке задачи: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка при получении количества затронутых строк: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("задача уже заблокирована другим воркером")
	}

	return nil
}

func (a *Analytics) UpdateStatsUpdateTask(taskID int, nextUpdateAt time.Time, intervalMinutes int) error {
	query, args, err := sq.Update("stats_update_tasks").
		Set("next_update_at", nextUpdateAt).
		Set("last_updated_at", time.Now()).
		Set("update_interval_minutes", intervalMinutes).
		Set("is_locked", false).
		Set("locked_at", nil).
		Set("locked_by", nil).
		Where(sq.Eq{"id": taskID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return fmt.Errorf("ошибка при формировании SQL-запроса для обновления задачи: %w", err)
	}

	_, err = a.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении задачи: %w", err)
	}

	return nil
}

func (a *Analytics) UnlockStatsUpdateTask(taskID int) error {
	query, args, err := sq.Update("stats_update_tasks").
		Set("is_locked", false).
		Set("locked_at", nil).
		Set("locked_by", nil).
		Where(sq.Eq{"id": taskID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return fmt.Errorf("ошибка при формировании SQL-запроса для разблокировки задачи: %w", err)
	}

	_, err = a.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("ошибка при разблокировке задачи: %w", err)
	}

	return nil
}

func (a *Analytics) DeleteStatsUpdateTask(taskID int) error {
	query, args, err := sq.Delete("stats_update_tasks").
		Where(sq.Eq{"id": taskID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return fmt.Errorf("ошибка при формировании SQL-запроса для удаления задачи: %w", err)
	}

	_, err = a.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("ошибка при удалении задачи: %w", err)
	}

	return nil
}

func (a *Analytics) GetCommentsCountByPeriod(postUnionID int, startDate, endDate time.Time) (int, error) {
	var query string
	var args []interface{}
	var err error

	if startDate.IsZero() {
		// Если startDate не указана, считаем от начала времени до endDate
		query, args, err = sq.Select("COUNT(*)").
			From("post_comment").
			Where(sq.Eq{"post_union_id": postUnionID}).
			Where(sq.LtOrEq{"created_at": endDate}).
			PlaceholderFormat(sq.Dollar).
			ToSql()
	} else {
		// Обычный случай - считаем за период
		query, args, err = sq.Select("COUNT(*)").
			From("post_comment").
			Where(sq.Eq{"post_union_id": postUnionID}).
			Where(sq.GtOrEq{"created_at": startDate}).
			Where(sq.LtOrEq{"created_at": endDate}).
			PlaceholderFormat(sq.Dollar).
			ToSql()
	}

	if err != nil {
		return 0, fmt.Errorf("ошибка при формировании SQL-запроса для подсчета комментариев за период: %w", err)
	}

	var count int
	err = a.db.Get(&count, query, args...)
	if err != nil {
		return 0, fmt.Errorf("ошибка при подсчете комментариев за период: %w", err)
	}

	return count, nil
}

func (a *Analytics) GetUserKPI(userID int, startDate, endDate time.Time) (*entity.UserKPI, error) {
	// Получаем все посты пользователя за указанный период
	queryPosts := `
		SELECT id
		FROM post_union
		WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
	`
	var postIDs []int
	err := a.db.Select(&postIDs, queryPosts, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get posts for user: %w", err)
	}

	if len(postIDs) == 0 {
		return &entity.UserKPI{UserID: userID, KPI: 0, Views: 0, Reactions: 0, Comments: 0}, nil
	}

	// Инициализируем метрики
	var totalViews, totalReactions, totalComments int
	// Считаем статистику по каждому посту
	for _, postID := range postIDs {
		// Получаем последние (актуальные) просмотры и реакции из post_platform_stats_history
		// Для каждой платформы берем самую последнюю запись по времени, затем суммируем по платформам
		queryStats := `
			SELECT 
				COALESCE(SUM(views), 0) AS views, 
				COALESCE(SUM(reactions), 0) AS reactions
			FROM (
				SELECT 
					views,
					reactions,
					ROW_NUMBER() OVER (PARTITION BY platform ORDER BY recorded_at DESC) as rn
				FROM post_platform_stats_history
				WHERE post_union_id = $1 AND recorded_at >= $2 AND recorded_at <= $3
			) ranked_stats
			WHERE rn = 1
		`
		var views, reactions int
		err := a.db.QueryRow(queryStats, postID, startDate, endDate).Scan(&views, &reactions)
		if err != nil {
			return nil, fmt.Errorf("failed to get stats for post %d: %w", postID, err)
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
			return nil, fmt.Errorf("failed to get comments for post %d: %w", postID, err)
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

	return &entity.UserKPI{
		UserID:    userID,
		KPI:       kpi,
		Views:     totalViews,
		Reactions: totalReactions,
		Comments:  totalComments,
	}, nil
}

func (a *Analytics) CompareUserKPI(userIDs []int, startDate, endDate time.Time) (map[int]*entity.UserKPI, error) {
	result := make(map[int]*entity.UserKPI)

	for _, userID := range userIDs {
		kpi, err := a.GetUserKPI(userID, startDate, endDate)
		if err != nil {
			return nil, fmt.Errorf("failed to get KPI for user %d: %w", userID, err)
		}
		result[userID] = kpi
	}

	return result, nil
}
