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
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var avatarMediafileID *int

	// Получаем ID аватара, если он есть
	if comment.AvatarMediaFile != nil {
		avatarMediafileID = &comment.AvatarMediaFile.ID
	}

	_, err = tx.Exec(`
        UPDATE post_comment
        SET team_id = $1, platform = $2, post_platform_id = $3, user_platform_id = $4, comment_platform_id = $5,
            full_name = $6, username = $7, avatar_mediafile_id = $8, text = $9,
            reply_to_comment_id = $10, is_team_reply = $11, created_at = $12, marked_as_ticket = $13
        WHERE id = $14
    `,
		comment.TeamID,
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
		comment.ID,
	)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (c *Comment) GetCommentInfoByPlatformID(platformID int, platform string) (*entity.Comment, error) {
	row := c.db.QueryRowx(`
        SELECT id, team_id, post_union_id, platform, post_platform_id, user_platform_id, comment_platform_id,
               full_name, username, avatar_mediafile_id, text, reply_to_comment_id, is_team_reply, created_at,
               marked_as_ticket
        FROM post_comment
        WHERE comment_platform_id = $1 AND platform = $2
    `, platformID, platform)

	var comment entity.Comment
	var avatarMediafileID *int

	err := row.Scan(
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
	)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, repo.ErrCommentNotFound
	case err != nil:
		return nil, err
	}

	if avatarMediafileID != nil {
		avatarRow := c.db.QueryRowx(`
		   SELECT id, file_path, file_type, uploaded_by_user_id, created_at
		   FROM mediafile
		   WHERE id = $1
		`, *avatarMediafileID)

		comment.AvatarMediaFile = &entity.Upload{}
		if err := avatarRow.StructScan(comment.AvatarMediaFile); err != nil {
			return nil, err
		}
	}

	// Загружаем вложения комментария
	attachmentsRows, err := c.db.Queryx(`
  SELECT m.id, m.file_path, m.file_type, m.uploaded_by_user_id, m.created_at
  FROM post_comment_attachment pca
  JOIN mediafile m ON pca.mediafile_id = m.id
  WHERE pca.comment_id = $1
`, comment.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = attachmentsRows.Close() }()
	comment.Attachments = make([]*entity.Upload, 0)
	for attachmentsRows.Next() {
		upload := &entity.Upload{}
		if err := attachmentsRows.StructScan(upload); err != nil {
			return nil, err
		}
		comment.Attachments = append(comment.Attachments, upload)
	}

	if err := attachmentsRows.Err(); err != nil {
		return nil, err
	}
	return &comment, nil
}

func (c *Comment) GetLastComments(postUnionID int, limit int) ([]*entity.JustTextComment, error) {
	rows, err := c.db.Queryx(`
		SELECT text
		FROM post_comment
		WHERE post_union_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, postUnionID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var comments []*entity.JustTextComment
	for rows.Next() {
		var comment entity.JustTextComment
		if err := rows.Scan(&comment.Text); err != nil {
			return nil, err
		}
		comments = append(comments, &comment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return comments, nil
}

func (c *Comment) AddComment(comment *entity.Comment) (int, error) {
	tx, err := c.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var commentID int
	var avatarMediafileID *int

	if comment.AvatarMediaFile != nil {
		avatarMediafileID = &comment.AvatarMediaFile.ID
	}

	row := tx.QueryRow(`
        INSERT INTO post_comment (team_id, post_union_id, platform, post_platform_id, user_platform_id, comment_platform_id,
                                full_name, username, avatar_mediafile_id, text, reply_to_comment_id, is_team_reply, created_at,
                                marked_as_ticket)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
        RETURNING id
    `,
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
	)
	err = row.Scan(&commentID)
	if err != nil {
		return 0, err
	}

	// Добавление вложений, если они есть
	if len(comment.Attachments) > 0 {
		for _, attachment := range comment.Attachments {
			_, err := tx.Exec(`
				INSERT INTO post_comment_attachment (comment_id, mediafile_id)
				VALUES ($1, $2)
			`, commentID, attachment.ID)
			if err != nil {
				return 0, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return commentID, nil
}

func (c *Comment) GetComments(
	teamID int,
	postUnionID int,
	offset time.Time,
	before bool,
	limit int,
	markedAsTicket *bool,
) ([]*entity.Comment, error) {
	var comparator string
	var sortOrder string
	markedAsTicketCondition := ""

	if before {
		// Получить комментарии ДО offset
		comparator = "<"
		sortOrder = "DESC" // Сначала новые
	} else {
		// Получить комментарии ПОСЛЕ offset
		comparator = ">"
		sortOrder = "ASC" // Сначала более старые
	}
	if markedAsTicket != nil {
		if *markedAsTicket {
			markedAsTicketCondition = "AND marked_as_ticket = true"
		} else {
			markedAsTicketCondition = "AND marked_as_ticket = false"
		}
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
		marked_as_ticket
    FROM post_comment
    WHERE ($1 = 0 OR team_id = $1)
	  AND ($1 = 0 OR "post_union_id" = $2)
      AND reply_to_comment_id = 0
      AND created_at %s $3
      %s
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
		marked_as_ticket
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
		pc.marked_as_ticket
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
	marked_as_ticket
FROM comment_tree
ORDER BY CASE WHEN reply_to_comment_id = 0 THEN 0 ELSE 1 END, created_at DESC
`, comparator, markedAsTicketCondition, sortOrder)

	rows, err := c.db.Queryx(query, teamID, postUnionID, offset, limit)
	if err != nil {
		return nil, err
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
		); err != nil {
			return nil, err
		}

		// Загружаем аватар, если он есть
		if avatarMediafileID != nil {
			avatarRow := c.db.QueryRowx(`
    SELECT id, file_path, file_type, uploaded_by_user_id, created_at
    FROM mediafile
    WHERE id = $1
   `, *avatarMediafileID)

			comment.AvatarMediaFile = &entity.Upload{}
			if err := avatarRow.StructScan(comment.AvatarMediaFile); err != nil {
				return nil, err
			}
		}

		// Загружаем вложения комментария
		attachmentsRows, err := c.db.Queryx(`
   SELECT m.id, m.file_path, m.file_type, m.uploaded_by_user_id, m.created_at
   FROM post_comment_attachment pca
   JOIN mediafile m ON pca.mediafile_id = m.id
   WHERE pca.comment_id = $1
  `, comment.ID)
		if err != nil {
			return nil, err
		}

		comment.Attachments = make([]*entity.Upload, 0)
		for attachmentsRows.Next() {
			upload := &entity.Upload{}
			if err := attachmentsRows.StructScan(upload); err != nil {
				_ = attachmentsRows.Close()
				return nil, err
			}
			comment.Attachments = append(comment.Attachments, upload)
		}
		_ = attachmentsRows.Close()

		comments = append(comments, &comment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return comments, nil
}

func (c *Comment) GetCommentInfo(commentID int) (*entity.Comment, error) {
	// Получаем основную информацию о комментарии
	row := c.db.QueryRowx(`
        SELECT id, team_id, post_union_id, platform, post_platform_id, user_platform_id, comment_platform_id,
               full_name, username, avatar_mediafile_id, text, reply_to_comment_id, is_team_reply, created_at,
               marked_as_ticket
        FROM post_comment
        WHERE id = $1
    `, commentID)

	var comment entity.Comment
	var avatarMediafileID *int

	err := row.Scan(
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
	)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, repo.ErrCommentNotFound
	case err != nil:
		return nil, err
	}

	// Загружаем аватар, если он есть
	if avatarMediafileID != nil {
		avatarRow := c.db.QueryRowx(`
   SELECT id, file_path, file_type, uploaded_by_user_id, created_at
   FROM mediafile
   WHERE id = $1
  `, *avatarMediafileID)

		comment.AvatarMediaFile = &entity.Upload{}
		if err := avatarRow.StructScan(comment.AvatarMediaFile); err != nil {
			return nil, err
		}
	}

	// Загружаем вложения комментария
	attachmentsRows, err := c.db.Queryx(`
  SELECT m.id, m.file_path, m.file_type, m.uploaded_by_user_id, m.created_at
  FROM post_comment_attachment pca
  JOIN mediafile m ON pca.mediafile_id = m.id
  WHERE pca.comment_id = $1
 `, comment.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = attachmentsRows.Close() }()

	comment.Attachments = make([]*entity.Upload, 0)
	for attachmentsRows.Next() {
		upload := &entity.Upload{}
		if err := attachmentsRows.StructScan(upload); err != nil {
			return nil, err
		}
		comment.Attachments = append(comment.Attachments, upload)
	}
	if err := attachmentsRows.Err(); err != nil {
		return nil, err
	}

	return &comment, nil
}

func (c *Comment) DeleteComment(commentID int) error {
	// Удаляем сам комментарий
	// Связанные записи в post_comment_attachment будут удалены автоматически благодаря ON DELETE CASCADE в схеме БД
	result, err := c.db.Exec("DELETE FROM post_comment WHERE id = $1", commentID)
	if err != nil {
		return err
	}

	// Проверяем, был ли удален комментарий
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return repo.ErrCommentNotFound
	}

	return nil
}
