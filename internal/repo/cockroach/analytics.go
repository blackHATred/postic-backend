package cockroach

import (
	"database/sql"
	"errors"
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
        SELECT team_id, post_union_id, platform, views, reactions, last_update
        FROM post_platform_stats
        WHERE post_union_id = $1 AND platform = $2
    `

	var stats entity.PostPlatformStats
	err := a.db.Get(&stats, query, postUnionID, platform)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repo.ErrPostPlatformStatsNotFound
		}
		return nil, err
	}

	// Получаем количество комментариев для поста
	comments, err := a.CommentsCount(stats.PostUnionID)
	if err != nil {
		return nil, err
	}
	stats.Comments = comments

	return &stats, nil
}

func (a *Analytics) GetPostPlatformStatsByPeriod(startDate, endDate time.Time, platform string) ([]*entity.PostPlatformStats, error) {
	query := `
        SELECT pps.team_id, pps.post_union_id, pps.platform, pps.views, pps.reactions, pps.last_update
        FROM post_platform_stats pps
        JOIN post_union pu ON pps.post_union_id = pu.id
        WHERE pps.platform = $1 AND pu.created_at BETWEEN $2 AND $3
    `

	var stats []*entity.PostPlatformStats
	err := a.db.Select(&stats, query, platform, startDate, endDate)
	if err != nil {
		return nil, err
	}

	// Для каждого поста получаем количество комментариев
	for _, stat := range stats {
		comments, err := a.CommentsCount(stat.PostUnionID)
		if err != nil {
			return nil, err
		}
		stat.Comments = comments
	}

	return stats, nil
}

func (a *Analytics) EditPostPlatformStats(stats *entity.PostPlatformStats) error {
	query := `
        UPDATE post_platform_stats
        SET views = $1, reactions = $2, last_update = $3
        WHERE post_union_id = $4 AND platform = $5
    `

	result, err := a.db.Exec(query,
		stats.Views,
		stats.Reactions,
		time.Now(),
		stats.PostUnionID,
		stats.Platform)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return repo.ErrPostPlatformStatsNotFound
	}

	return nil
}

func (a *Analytics) AddPostPlatformStats(stats *entity.PostPlatformStats) error {
	query := `
        INSERT INTO post_platform_stats (team_id, post_union_id, platform, views, reactions, last_update)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (post_union_id, platform) DO NOTHING
    `

	_, err := a.db.Exec(query,
		stats.TeamID,
		stats.PostUnionID,
		stats.Platform,
		stats.Views,
		stats.Reactions,
		time.Now())

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
