package cockroach

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"time"
)

type PostDB struct {
	db *sqlx.DB
}

func NewPost(db *sqlx.DB) repo.Post {
	return &PostDB{db: db}
}

func (p *PostDB) GetPostUnions(teamID int, offset time.Time, before bool, limit int, filter *string) ([]*entity.PostUnion, error) {
	comparator := ">"
	sortOrder := "ASC"
	filterCondition := "AND 1=1"

	if before {
		comparator = "<"
		sortOrder = "DESC"
	}
	if filter != nil {
		switch *filter {
		case "scheduled":
			filterCondition = "AND pub_datetime IS NOT NULL AND pub_datetime > NOW()"
		case "published":
			filterCondition = "AND (pub_datetime IS NULL OR pub_datetime <= NOW())"
		}
	}

	query := fmt.Sprintf(`
        SELECT id, user_id, team_id, text, platforms, created_at, pub_datetime
        FROM post_union
        WHERE team_id = $1 AND created_at %s $2 %s
        ORDER BY created_at %s
        LIMIT $3
	`, comparator, filterCondition, sortOrder)

	rows, err := p.db.Queryx(query, teamID, offset, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var postUnions []*entity.PostUnion
	for rows.Next() {
		var post entity.PostUnion
		err := rows.Scan(
			&post.ID,
			&post.UserID,
			&post.TeamID,
			&post.Text,
			pq.Array(&post.Platforms),
			&post.CreatedAt,
			&post.PubDate,
		)
		if err != nil {
			return nil, err
		}
		postUnions = append(postUnions, &post)
	}

	// Получаем attachments на каждый пост
	for _, post := range postUnions {
		attachmentQuery := `
            SELECT m.id, m.file_path, m.file_type, m.uploaded_by_user_id, m.created_at
            FROM post_union_mediafile pum
            JOIN mediafile m ON pum.mediafile_id = m.id
            WHERE pum.post_union_id = $1
        `

		var attachments []*entity.Upload
		err = p.db.Select(&attachments, attachmentQuery, post.ID)
		if err != nil {
			return nil, err
		}

		post.Attachments = attachments
	}

	return postUnions, nil
}

func (p *PostDB) GetPostUnion(postUnionID int) (*entity.PostUnion, error) {
	var post entity.PostUnion
	query := `
		SELECT id, user_id, team_id, text, platforms, created_at, pub_datetime
		FROM post_union
		WHERE id = $1
	`

	row := p.db.QueryRow(query, postUnionID)
	err := row.Scan(
		&post.ID,
		&post.UserID,
		&post.TeamID,
		&post.Text,
		pq.Array(&post.Platforms),
		&post.CreatedAt,
		&post.PubDate,
	)

	if err != nil {
		return nil, err
	}

	// Получаем attachments на пост
	attachmentQuery := `
		SELECT m.id, m.file_path, m.file_type, m.uploaded_by_user_id, m.created_at
		FROM post_union_mediafile pum
		JOIN mediafile m ON pum.mediafile_id = m.id
		WHERE pum.post_union_id = $1
	`
	var attachments []*entity.Upload
	err = p.db.Select(&attachments, attachmentQuery, postUnionID)
	if err != nil {
		return nil, err
	}
	post.Attachments = attachments

	return &post, nil
}

func (p *PostDB) AddPostUnion(union *entity.PostUnion) (int, error) {
	query := `
		INSERT INTO post_union (user_id, team_id, text, platforms, created_at, pub_datetime)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	var postUnionID int
	err := p.db.QueryRow(query, union.UserID, union.TeamID, union.Text, pq.Array(union.Platforms), union.CreatedAt, union.PubDate).Scan(&postUnionID)
	if err != nil {
		return 0, err
	}

	// Добавление прикрепленных медиафайлов
	for _, attachment := range union.Attachments {
		attachmentQuery := `
			INSERT INTO post_union_mediafile (post_union_id, mediafile_id)
			VALUES ($1, $2)
		`
		_, err := p.db.Exec(attachmentQuery, postUnionID, attachment.ID)
		if err != nil {
			return postUnionID, err
		}
	}

	return postUnionID, nil
}

func (p *PostDB) EditPostUnion(union *entity.PostUnion) error {
	// Начинаем транзакцию на время редактирования нескольких таблиц
	tx, err := p.db.Beginx()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Обновляем запись
	query := `
        UPDATE post_union
        SET text = $1, platforms = $2, pub_datetime = $3
        WHERE id = $4
    `
	_, err = tx.Exec(query, union.Text, pq.Array(union.Platforms), union.PubDate, union.ID)
	if err != nil {
		return err
	}

	/*
			// Удаляем существующие аттачи
			deleteQuery := `
		        DELETE FROM post_union_mediafile
		        WHERE post_union_id = $1
		    `
			_, err = tx.Exec(deleteQuery, union.ID)
			if err != nil {
				return err
			}

			// Добавляем новые аттачи
			for _, attachment := range union.Attachments {
				attachmentQuery := `
		            INSERT INTO post_union_mediafile (post_union_id, mediafile_id)
		            VALUES ($1, $2)
		        `
				_, err = tx.Exec(attachmentQuery, union.ID, attachment.ID)
				if err != nil {
					return err
				}
			}
	*/

	// Коммитим
	return tx.Commit()
}

func (p *PostDB) GetScheduledPosts(status string, offset time.Time, before bool, limit int) ([]*entity.ScheduledPost, error) {
	var comparator string
	var sortOrder string

	if before {
		comparator = "<"
		sortOrder = "ASC"
	} else {
		comparator = ">"
		sortOrder = "DESC"
	}

	query := fmt.Sprintf(`
        SELECT post_union_id, scheduled_at, status, created_at
        FROM scheduled_post
        WHERE status = $1 AND scheduled_at %s $2
        ORDER BY scheduled_at %s
        LIMIT $3
    `, comparator, sortOrder)

	var scheduledPosts []*entity.ScheduledPost
	err := p.db.Select(&scheduledPosts, query, status, offset, limit)
	if err != nil {
		return nil, err
	}

	return scheduledPosts, nil
}

func (p *PostDB) GetScheduledPost(postUnionID int) (*entity.ScheduledPost, error) {
	query := `
        SELECT post_union_id, scheduled_at, status, created_at
        FROM scheduled_post
        WHERE post_union_id = $1
    `

	var scheduledPost entity.ScheduledPost
	err := p.db.Get(&scheduledPost, query, postUnionID)
	if err != nil {
		return nil, err
	}

	return &scheduledPost, nil
}

func (p *PostDB) AddScheduledPost(scheduledPost *entity.ScheduledPost) (int, error) {
	query := `
        INSERT INTO scheduled_post (post_union_id, scheduled_at, status, created_at)
        VALUES ($1, $2, $3, $4)
        RETURNING post_union_id
    `

	createdAt := scheduledPost.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	var postUnionID int
	err := p.db.QueryRow(
		query,
		scheduledPost.PostUnionID,
		scheduledPost.ScheduledAt,
		scheduledPost.Status,
		createdAt,
	).Scan(&postUnionID)

	if err != nil {
		return 0, err
	}

	return postUnionID, nil
}

func (p *PostDB) EditScheduledPost(scheduledPost *entity.ScheduledPost) error {
	query := `
        UPDATE scheduled_post
        SET scheduled_at = $1, status = $2
        WHERE post_union_id = $3
    `
	_, err := p.db.Exec(query, scheduledPost.ScheduledAt, scheduledPost.Status, scheduledPost.PostUnionID)
	return err
}

func (p *PostDB) DeleteScheduledPost(postUnionID int) error {
	query := `
        DELETE FROM scheduled_post
        WHERE post_union_id = $1
    `
	_, err := p.db.Exec(query, postUnionID)
	return err
}

func (p *PostDB) GetPostActions(postUnionID int) ([]int, error) {
	query := `
        SELECT id
        FROM post_action
        WHERE post_union_id = $1
        ORDER BY created_at DESC
    `

	var actionIDs []int
	err := p.db.Select(&actionIDs, query, postUnionID)
	if err != nil {
		return nil, err
	}

	return actionIDs, nil
}

func (p *PostDB) GetPostAction(postActionID int) (*entity.PostAction, error) {
	var postAction entity.PostAction
	query := `
		SELECT id, post_union_id, op, platform, status, error_message, created_at
		FROM post_action
		WHERE id = $1
	`
	err := p.db.Get(&postAction, query, postActionID)
	if err != nil {
		return nil, err
	}
	return &postAction, nil
}

func (p *PostDB) AddPostAction(postAction *entity.PostAction) (int, error) {
	query := `
		INSERT INTO post_action (post_union_id, op, platform, status, error_message, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	var postActionID int
	err := p.db.QueryRow(query, postAction.PostUnionID, postAction.Operation, postAction.Platform, postAction.Status, postAction.ErrMessage, postAction.CreatedAt).Scan(&postActionID)
	if err != nil {
		return 0, err
	}
	return postActionID, nil
}

func (p *PostDB) EditPostAction(postAction *entity.PostAction) error {
	query := `
		UPDATE post_action
		SET post_union_id = $1, op = $2, platform = $3, status = $4, error_message = $5, created_at = $6
		WHERE id = $7
	`
	_, err := p.db.Exec(query, postAction.PostUnionID, postAction.Operation, postAction.Platform, postAction.Status, postAction.ErrMessage, postAction.CreatedAt, postAction.ID)
	return err
}

func (p *PostDB) GetPostPlatform(postUnionID int, platform string) (*entity.PostPlatform, error) {
	var postPlatform entity.PostPlatform
	query := `
		SELECT id, post_union_id, post_id, platform
		FROM post_platform
		WHERE post_union_id = $1 AND platform = $2
	`
	err := p.db.Get(&postPlatform, query, postUnionID, platform)
	if err != nil {
		return nil, err
	}

	// Если это Telegram, получаем связанные сообщения из медиа-группы
	if platform == "tg" {
		groupQuery := `
			SELECT tg_post_id, post_platform_id
			FROM tg_post_platform_group
			WHERE post_platform_id = $1
		`
		var tgGroups []entity.TgPostPlatformGroup
		err = p.db.Select(&tgGroups, groupQuery, postPlatform.ID)
		if err != nil {
			return nil, err
		}
		postPlatform.TgPostPlatformGroup = tgGroups
	}

	return &postPlatform, nil
}

func (p *PostDB) GetPostPlatformByPlatformPostID(platformID int, platform string) (*entity.PostPlatform, error) {
	var postPlatform entity.PostPlatform
	query := `
		SELECT id, post_union_id, post_id, platform
		FROM post_platform
		WHERE post_id = $1 AND platform = $2
	`
	err := p.db.Get(&postPlatform, query, platformID, platform)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, repo.ErrPostPlatformNotFound
	case err != nil:
		return nil, err
	}

	// Если это Telegram, получаем связанные сообщения из медиа-группы
	if platform == "tg" {
		groupQuery := `
			SELECT tg_post_id, post_platform_id
			FROM tg_post_platform_group
			WHERE post_platform_id = $1
		`
		var tgGroups []entity.TgPostPlatformGroup
		err = p.db.Select(&tgGroups, groupQuery, postPlatform.ID)
		if err != nil {
			return nil, err
		}
		postPlatform.TgPostPlatformGroup = tgGroups
	}

	return &postPlatform, nil
}

func (p *PostDB) AddPostPlatform(postPlatform *entity.PostPlatform) (int, error) {
	query := `
		INSERT INTO post_platform (post_union_id, post_id, platform)
		VALUES ($1, $2, $3)
		RETURNING id
	`
	var postPlatformID int
	err := p.db.QueryRow(query, postPlatform.PostUnionId, postPlatform.PostId, postPlatform.Platform).Scan(&postPlatformID)
	if err != nil {
		return 0, err
	}

	// Добавляем связанные сообщения для Telegram, если они есть
	if postPlatform.Platform == "tg" && len(postPlatform.TgPostPlatformGroup) > 0 {
		for _, tgGroup := range postPlatform.TgPostPlatformGroup {
			groupQuery := `
				INSERT INTO tg_post_platform_group (tg_post_id, post_platform_id)
				VALUES ($1, $2)
			`
			_, err := p.db.Exec(groupQuery, tgGroup.TgPostID, postPlatformID)
			if err != nil {
				return postPlatformID, err
			}
		}
	}

	return postPlatformID, nil
}

func (p *PostDB) DeletePlatformFromPostUnion(postUnionID int, platform string) error {
	// Начинаем транзакцию
	tx, err := p.db.Beginx()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Получаем текущее состояние с блокировкой
	var platforms []string
	query := `
		SELECT platforms
		FROM post_union
		WHERE id = $1
		FOR UPDATE
	`
	err = tx.QueryRow(query, postUnionID).Scan(pq.Array(&platforms))
	if errors.Is(err, sql.ErrNoRows) {
		tx.Rollback()
		return nil // Запись уже удалена, ничего не делаем
	}
	if err != nil {
		return err
	}

	// Удаляем указанную платформу из массива
	newPlatforms := make([]string, 0, len(platforms))
	platformFound := false
	for _, p := range platforms {
		if p != platform {
			newPlatforms = append(newPlatforms, p)
		} else {
			platformFound = true
		}
	}

	// Если платформа не найдена, ничего не делаем
	if !platformFound {
		tx.Rollback()
		return nil
	}

	// Если массив пустой, удаляем запись полностью
	if len(newPlatforms) == 0 {
		deleteQuery := `
			DELETE FROM post_union
			WHERE id = $1
		`
		_, err = tx.Exec(deleteQuery, postUnionID)
		if err != nil {
			return err
		}
	} else {
		// Обновляем запись с новым массивом
		updateQuery := `
			UPDATE post_union
			SET platforms = $1
			WHERE id = $2
		`
		_, err = tx.Exec(updateQuery, pq.Array(newPlatforms), postUnionID)
		if err != nil {
			return err
		}
	}

	// Коммитим транзакцию
	return tx.Commit()
}

func (p *PostDB) DeletePostPlatform() error {
	//TODO implement me
	panic("implement me")
}
