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

type Comment struct {
	db *sqlx.DB
}

func NewComment(db *sqlx.DB) repo.Comment {
	return &Comment{
		db: db,
	}
}

func (c *Comment) EditComment(comment *entity.Comment) error {
	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var avatarMediafileID *int

	// Получаем ID аватара, если он есть
	if comment.AvatarMediaFile != nil {
		avatarMediafileID = &comment.AvatarMediaFile.ID
	}

	query, args, err := sq.Update("post_comment").
		Set("team_id", comment.TeamID).
		Set("platform", comment.Platform).
		Set("post_platform_id", comment.PostPlatformID).
		Set("user_platform_id", comment.UserPlatformID).
		Set("comment_platform_id", comment.CommentPlatformID).
		Set("full_name", comment.FullName).
		Set("username", comment.Username).
		Set("avatar_mediafile_id", avatarMediafileID).
		Set("text", comment.Text).
		Set("reply_to_comment_id", comment.ReplyToCommentID).
		Set("is_team_reply", comment.IsTeamReply).
		Set("created_at", comment.CreatedAt).
		Set("marked_as_ticket", comment.MarkedAsTicket).
		Where(sq.Eq{"id": comment.ID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return fmt.Errorf("ошибка при формировании SQL-запроса для обновления комментария: %w", err)
	}

	_, err = tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении комментария: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ошибка при коммите транзакции: %w", err)
	}

	return nil
}

func (c *Comment) GetCommentByPlatformID(platformID int, platform string) (*entity.Comment, error) {
	query, args, err := sq.Select(
		"id", "team_id", "post_union_id", "platform", "post_platform_id",
		"user_platform_id", "comment_platform_id", "full_name", "username",
		"avatar_mediafile_id", "text", "reply_to_comment_id", "is_team_reply",
		"created_at", "marked_as_ticket", "is_deleted",
	).
		From("post_comment").
		Where(sq.Eq{"comment_platform_id": platformID}).
		Where(sq.Eq{"platform": platform}).
		Where(sq.Eq{"is_deleted": false}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("ошибка при формировании SQL-запроса для получения комментария: %w", err)
	}

	var comment entity.Comment
	var avatarMediafileID *int

	err = c.db.QueryRow(query, args...).Scan(
		&comment.ID,
		&comment.TeamID,
		&comment.PostUnionID,
		&comment.Platform,
		&comment.PostPlatformID,
		&comment.UserPlatformID,
		&comment.CommentPlatformID,
		&comment.FullName,
		&comment.Username,
		&avatarMediafileID,
		&comment.Text,
		&comment.ReplyToCommentID,
		&comment.IsTeamReply,
		&comment.CreatedAt,
		&comment.MarkedAsTicket,
		&comment.IsDeleted,
	)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, repo.ErrCommentNotFound
	case err != nil:
		return nil, fmt.Errorf("ошибка при получении комментария: %w", err)
	}

	if avatarMediafileID != nil {
		avatarQuery, avatarArgs, err := sq.Select(
			"id", "file_path", "file_type", "uploaded_by_user_id", "created_at",
		).
			From("mediafile").
			Where(sq.Eq{"id": *avatarMediafileID}).
			PlaceholderFormat(sq.Dollar).
			ToSql()

		if err != nil {
			return nil, fmt.Errorf("ошибка при формировании SQL-запроса для получения аватара: %w", err)
		}

		comment.AvatarMediaFile = &entity.Upload{}
		avatarRow := c.db.QueryRowx(avatarQuery, avatarArgs...)
		if err := avatarRow.StructScan(comment.AvatarMediaFile); err != nil {
			return nil, fmt.Errorf("ошибка при сканировании аватара: %w", err)
		}
	}

	// Загружаем вложения комментария
	attachmentsQuery, attachmentsArgs, err := sq.Select(
		"m.id", "m.file_path", "m.file_type", "m.uploaded_by_user_id", "m.created_at",
	).
		From("post_comment_attachment pca").
		Join("mediafile m ON pca.mediafile_id = m.id").
		Where(sq.Eq{"pca.comment_id": comment.ID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("ошибка при формировании SQL-запроса для получения вложений: %w", err)
	}

	attachmentsRows, err := c.db.Queryx(attachmentsQuery, attachmentsArgs...)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении вложений: %w", err)
	}
	defer func() { _ = attachmentsRows.Close() }()

	comment.Attachments = make([]*entity.Upload, 0)
	for attachmentsRows.Next() {
		upload := &entity.Upload{}
		if err := attachmentsRows.StructScan(upload); err != nil {
			return nil, fmt.Errorf("ошибка при сканировании вложения: %w", err)
		}
		comment.Attachments = append(comment.Attachments, upload)
	}

	if err := attachmentsRows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке вложений: %w", err)
	}
	return &comment, nil
}

func (c *Comment) GetLastComments(postUnionID int, limit int) ([]*entity.JustTextComment, error) {
	query, args, err := sq.Select("text").
		From("post_comment").
		Where(sq.Eq{"post_union_id": postUnionID}).
		Where(sq.Eq{"is_deleted": false}).
		OrderBy("created_at DESC").
		Limit(uint64(limit)).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("ошибка при формировании SQL-запроса для получения последних комментариев: %w", err)
	}

	rows, err := c.db.Queryx(query, args...)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении последних комментариев: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var comments []*entity.JustTextComment
	for rows.Next() {
		var comment entity.JustTextComment
		if err := rows.Scan(&comment.Text); err != nil {
			return nil, fmt.Errorf("ошибка при сканировании текста комментария: %w", err)
		}
		comments = append(comments, &comment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке результатов запроса: %w", err)
	}

	return comments, nil
}

func (c *Comment) AddComment(comment *entity.Comment) (int, error) {
	tx, err := c.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var commentID int
	var avatarMediafileID *int

	if comment.AvatarMediaFile != nil {
		avatarMediafileID = &comment.AvatarMediaFile.ID
	}

	insertQuery, insertArgs, err := sq.Insert("post_comment").
		Columns(
			"team_id", "post_union_id", "platform", "post_platform_id",
			"user_platform_id", "comment_platform_id", "full_name", "username",
			"avatar_mediafile_id", "text", "reply_to_comment_id", "is_team_reply",
			"created_at", "marked_as_ticket",
		).
		Values(
			comment.TeamID,
			comment.PostUnionID,
			comment.Platform,
			comment.PostPlatformID,
			comment.UserPlatformID,
			comment.CommentPlatformID,
			comment.FullName,
			comment.Username,
			avatarMediafileID,
			comment.Text,
			comment.ReplyToCommentID,
			comment.IsTeamReply,
			comment.CreatedAt,
			comment.MarkedAsTicket,
		).
		Suffix("RETURNING id").
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return 0, fmt.Errorf("ошибка при формировании SQL-запроса для добавления комментария: %w", err)
	}

	err = tx.QueryRow(insertQuery, insertArgs...).Scan(&commentID)
	if err != nil {
		return 0, fmt.Errorf("ошибка при добавлении комментария: %w", err)
	}

	// Добавление вложений, если они есть
	if len(comment.Attachments) > 0 {
		for _, attachment := range comment.Attachments {
			attachQuery, attachArgs, err := sq.Insert("post_comment_attachment").
				Columns("comment_id", "mediafile_id").
				Values(commentID, attachment.ID).
				PlaceholderFormat(sq.Dollar).
				ToSql()

			if err != nil {
				return 0, fmt.Errorf("ошибка при формировании SQL-запроса для добавления вложения: %w", err)
			}

			_, err = tx.Exec(attachQuery, attachArgs...)
			if err != nil {
				return 0, fmt.Errorf("ошибка при добавлении вложения: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("ошибка при коммите транзакции: %w", err)
	}

	return commentID, nil
}

func (c *Comment) GetComments(
	teamID int,
	postUnionID int,
	offset time.Time,
	before bool,
	limit int,
) ([]*entity.Comment, error) {
	var comparator string
	var sortOrder string

	if before {
		// Получить комментарии ДО offset
		comparator = "<"
		sortOrder = "DESC" // Сначала новые
	} else {
		// Получить комментарии ПОСЛЕ offset
		comparator = ">"
		sortOrder = "ASC" // Сначала более старые
	}

	/*
		Этот запрос:
		Сначала выбирает только корневые комментарии (без ответов) с применением LIMIT
		Затем рекурсивно добавляет все ответы на эти комментарии любого уровня вложенности
		Сохраняет исходную логику сортировки, располагая родительские комментарии перед ответами
	*/
	query := fmt.Sprintf(
		`
WITH RECURSIVE top_level_comments AS (
    SELECT
        id,
        team_id,
        "post_union_id",
        platform,
        post_platform_id,
        user_platform_id,
        comment_platform_id,
        full_name,
        username,
        avatar_mediafile_id,
        text,
        reply_to_comment_id,
        is_team_reply,
        created_at,
		marked_as_ticket,
		is_deleted
    FROM post_comment
    WHERE ($1 = 0 OR team_id = $1)
	  AND ($2 = 0 OR "post_union_id" = $2)
      AND reply_to_comment_id = 0
      AND created_at %s $3
    ORDER BY created_at %s
    LIMIT $4
),
comment_tree AS (
    SELECT
        id,
        team_id,
        "post_union_id",
        platform,
        post_platform_id,
        user_platform_id,
        comment_platform_id,
        full_name,
        username,
        avatar_mediafile_id,
        text,
        reply_to_comment_id,
        is_team_reply,
        created_at,
		marked_as_ticket,
		is_deleted
    FROM top_level_comments

    UNION ALL

    SELECT
        pc.id,
        pc.team_id,
        pc."post_union_id",
        pc.platform,
        pc.post_platform_id,
        pc.user_platform_id,
        pc.comment_platform_id,
        pc.full_name,
        pc.username,
        pc.avatar_mediafile_id,
        pc.text,
        pc.reply_to_comment_id,
        pc.is_team_reply,
        pc.created_at,
		pc.marked_as_ticket,
		pc.is_deleted
    FROM post_comment pc
    JOIN comment_tree ct ON pc.reply_to_comment_id = ct.id
)
SELECT
    id,
    team_id,
    "post_union_id",
    platform,
    post_platform_id,
    user_platform_id,
    comment_platform_id,
    full_name,
    username,
    avatar_mediafile_id,
    text,
    reply_to_comment_id,
    is_team_reply,
    created_at,
	marked_as_ticket,
	is_deleted
FROM comment_tree
ORDER BY CASE WHEN reply_to_comment_id = 0 THEN 0 ELSE 1 END, created_at DESC
`, comparator, sortOrder)

	rows, err := c.db.Queryx(query, teamID, postUnionID, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении комментариев: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var comments []*entity.Comment
	for rows.Next() {
		var comment entity.Comment
		var avatarMediafileID *int

		if err := rows.Scan(
			&comment.ID,
			&comment.TeamID,
			&comment.PostUnionID,
			&comment.Platform,
			&comment.PostPlatformID,
			&comment.UserPlatformID,
			&comment.CommentPlatformID,
			&comment.FullName,
			&comment.Username,
			&avatarMediafileID,
			&comment.Text,
			&comment.ReplyToCommentID,
			&comment.IsTeamReply,
			&comment.CreatedAt,
			&comment.MarkedAsTicket,
			&comment.IsDeleted,
		); err != nil {
			return nil, fmt.Errorf("ошибка при сканировании комментария: %w", err)
		}

		// Загружаем аватар, если он есть
		if avatarMediafileID != nil {
			avatarQuery, avatarArgs, err := sq.Select(
				"id", "file_path", "file_type", "uploaded_by_user_id", "created_at",
			).
				From("mediafile").
				Where(sq.Eq{"id": *avatarMediafileID}).
				PlaceholderFormat(sq.Dollar).
				ToSql()

			if err != nil {
				return nil, fmt.Errorf("ошибка при формировании SQL-запроса для получения аватара: %w", err)
			}

			comment.AvatarMediaFile = &entity.Upload{}
			avatarRow := c.db.QueryRowx(avatarQuery, avatarArgs...)
			if err := avatarRow.StructScan(comment.AvatarMediaFile); err != nil {
				return nil, fmt.Errorf("ошибка при сканировании аватара: %w", err)
			}
		}

		// Загружаем вложения комментария
		attachmentsQuery, attachmentsArgs, err := sq.Select(
			"m.id", "m.file_path", "m.file_type", "m.uploaded_by_user_id", "m.created_at",
		).
			From("post_comment_attachment pca").
			Join("mediafile m ON pca.mediafile_id = m.id").
			Where(sq.Eq{"pca.comment_id": comment.ID}).
			PlaceholderFormat(sq.Dollar).
			ToSql()

		if err != nil {
			return nil, fmt.Errorf("ошибка при формировании SQL-запроса для получения вложений: %w", err)
		}

		attachmentsRows, err := c.db.Queryx(attachmentsQuery, attachmentsArgs...)
		if err != nil {
			return nil, fmt.Errorf("ошибка при получении вложений: %w", err)
		}

		comment.Attachments = make([]*entity.Upload, 0)
		for attachmentsRows.Next() {
			upload := &entity.Upload{}
			if err := attachmentsRows.StructScan(upload); err != nil {
				_ = attachmentsRows.Close()
				return nil, fmt.Errorf("ошибка при сканировании вложения: %w", err)
			}
			comment.Attachments = append(comment.Attachments, upload)
		}
		_ = attachmentsRows.Close()

		comments = append(comments, &comment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке результатов запроса: %w", err)
	}

	return comments, nil
}

func (c *Comment) GetComment(commentID int) (*entity.Comment, error) {
	// Получаем основную информацию о комментарии
	query, args, err := sq.Select(
		"id", "team_id", "post_union_id", "platform", "post_platform_id",
		"user_platform_id", "comment_platform_id", "full_name", "username",
		"avatar_mediafile_id", "text", "reply_to_comment_id", "is_team_reply",
		"created_at", "marked_as_ticket", "is_deleted",
	).
		From("post_comment").
		Where(sq.Eq{"id": commentID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("ошибка при формировании SQL-запроса для получения комментария: %w", err)
	}

	var comment entity.Comment
	var avatarMediafileID *int

	err = c.db.QueryRow(query, args...).Scan(
		&comment.ID,
		&comment.TeamID,
		&comment.PostUnionID,
		&comment.Platform,
		&comment.PostPlatformID,
		&comment.UserPlatformID,
		&comment.CommentPlatformID,
		&comment.FullName,
		&comment.Username,
		&avatarMediafileID,
		&comment.Text,
		&comment.ReplyToCommentID,
		&comment.IsTeamReply,
		&comment.CreatedAt,
		&comment.MarkedAsTicket,
		&comment.IsDeleted,
	)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, repo.ErrCommentNotFound
	case err != nil:
		return nil, fmt.Errorf("ошибка при получении комментария: %w", err)
	}

	// Загружаем аватар, если он есть
	if avatarMediafileID != nil {
		avatarQuery, avatarArgs, err := sq.Select(
			"id", "file_path", "file_type", "uploaded_by_user_id", "created_at",
		).
			From("mediafile").
			Where(sq.Eq{"id": *avatarMediafileID}).
			PlaceholderFormat(sq.Dollar).
			ToSql()

		if err != nil {
			return nil, fmt.Errorf("ошибка при формировании SQL-запроса для получения аватара: %w", err)
		}

		comment.AvatarMediaFile = &entity.Upload{}
		avatarRow := c.db.QueryRowx(avatarQuery, avatarArgs...)
		if err := avatarRow.StructScan(comment.AvatarMediaFile); err != nil {
			return nil, fmt.Errorf("ошибка при сканировании аватара: %w", err)
		}
	}

	// Загружаем вложения комментария
	attachmentsQuery, attachmentsArgs, err := sq.Select(
		"m.id", "m.file_path", "m.file_type", "m.uploaded_by_user_id", "m.created_at",
	).
		From("post_comment_attachment pca").
		Join("mediafile m ON pca.mediafile_id = m.id").
		Where(sq.Eq{"pca.comment_id": comment.ID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("ошибка при формировании SQL-запроса для получения вложений: %w", err)
	}

	attachmentsRows, err := c.db.Queryx(attachmentsQuery, attachmentsArgs...)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении вложений: %w", err)
	}
	defer func() { _ = attachmentsRows.Close() }()

	comment.Attachments = make([]*entity.Upload, 0)
	for attachmentsRows.Next() {
		upload := &entity.Upload{}
		if err := attachmentsRows.StructScan(upload); err != nil {
			return nil, fmt.Errorf("ошибка при сканировании вложения: %w", err)
		}
		comment.Attachments = append(comment.Attachments, upload)
	}
	if err := attachmentsRows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке результатов запроса: %w", err)
	}

	return &comment, nil
}

func (c *Comment) GetTicketComments(teamID int, offset time.Time, before bool, limit int) ([]*entity.Comment, error) {
	var comparator string
	var sortOrder string

	if before {
		// Получить комментарии ДО offset
		comparator = "<"
		sortOrder = "DESC" // Сначала новые
	} else {
		// Получить комментарии ПОСЛЕ offset
		comparator = ">"
		sortOrder = "ASC" // Сначала более старые
	}

	columns := []string{
		"id", "team_id", "post_union_id", "platform", "post_platform_id",
		"user_platform_id", "comment_platform_id", "full_name", "username",
		"avatar_mediafile_id", "text", "reply_to_comment_id", "is_team_reply",
		"created_at", "marked_as_ticket", "is_deleted",
	}

	// Создаем запрос с использованием squirrel, добавляя фильтр is_deleted = false
	query, args, err := sq.Select(columns...).
		From("post_comment").
		Where(sq.Eq{"team_id": teamID}).
		Where(fmt.Sprintf("created_at %s ?", comparator), offset).
		Where(sq.Eq{"marked_as_ticket": true}).
		Where(sq.Eq{"is_deleted": false}). // Добавляем фильтр, чтобы не возвращать удаленные комментарии
		OrderBy(fmt.Sprintf("created_at %s", sortOrder)).
		Limit(uint64(limit)).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("ошибка при формировании SQL-запроса для получения тикетов: %w", err)
	}

	rows, err := c.db.Queryx(query, args...)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении тикетов: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var comments []*entity.Comment
	for rows.Next() {
		var comment entity.Comment
		var avatarMediafileID *int

		if err := rows.Scan(
			&comment.ID,
			&comment.TeamID,
			&comment.PostUnionID,
			&comment.Platform,
			&comment.PostPlatformID,
			&comment.UserPlatformID,
			&comment.CommentPlatformID,
			&comment.FullName,
			&comment.Username,
			&avatarMediafileID,
			&comment.Text,
			&comment.ReplyToCommentID,
			&comment.IsTeamReply,
			&comment.CreatedAt,
			&comment.MarkedAsTicket,
			&comment.IsDeleted,
		); err != nil {
			return nil, fmt.Errorf("ошибка при сканировании тикета: %w", err)
		}

		// Загружаем аватар, если он есть
		if avatarMediafileID != nil {
			avatarQuery, avatarArgs, err := sq.Select(
				"id", "file_path", "file_type", "uploaded_by_user_id", "created_at",
			).
				From("mediafile").
				Where(sq.Eq{"id": *avatarMediafileID}).
				PlaceholderFormat(sq.Dollar).
				ToSql()

			if err != nil {
				return nil, fmt.Errorf("ошибка при формировании SQL-запроса для получения аватара: %w", err)
			}

			comment.AvatarMediaFile = &entity.Upload{}
			avatarRow := c.db.QueryRowx(avatarQuery, avatarArgs...)
			if err := avatarRow.StructScan(comment.AvatarMediaFile); err != nil {
				return nil, fmt.Errorf("ошибка при сканировании аватара: %w", err)
			}
		}

		// Загружаем вложения комментария
		attachmentsQuery, attachmentsArgs, err := sq.Select(
			"m.id", "m.file_path", "m.file_type", "m.uploaded_by_user_id", "m.created_at",
		).
			From("post_comment_attachment pca").
			Join("mediafile m ON pca.mediafile_id = m.id").
			Where(sq.Eq{"pca.comment_id": comment.ID}).
			PlaceholderFormat(sq.Dollar).
			ToSql()

		if err != nil {
			return nil, fmt.Errorf("ошибка при формировании SQL-запроса для получения вложений: %w", err)
		}

		attachmentsRows, err := c.db.Queryx(attachmentsQuery, attachmentsArgs...)
		if err != nil {
			return nil, fmt.Errorf("ошибка при получении вложений: %w", err)
		}

		comment.Attachments = make([]*entity.Upload, 0)
		for attachmentsRows.Next() {
			upload := &entity.Upload{}
			if err := attachmentsRows.StructScan(upload); err != nil {
				_ = attachmentsRows.Close()
				return nil, fmt.Errorf("ошибка при сканировании вложения: %w", err)
			}
			comment.Attachments = append(comment.Attachments, upload)
		}
		_ = attachmentsRows.Close()

		comments = append(comments, &comment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке результатов запроса: %w", err)
	}

	return comments, nil
}

func (c *Comment) DeleteComment(commentID int) error {
	// Вместо удаления комментария, помечаем его как удаленный
	query, args, err := sq.Update("post_comment").
		Set("is_deleted", true).
		Where(sq.Eq{"id": commentID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return fmt.Errorf("ошибка при формировании SQL-запроса для пометки комментария как удаленного: %w", err)
	}

	_, err = c.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("ошибка при пометке комментария как удаленного: %w", err)
	}

	return nil
}
