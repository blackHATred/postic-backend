package cockroach

import (
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
		SET platform = $1, post_platform_id = $2, user_platform_id = $3, comment_platform_id = $4,
		    full_name = $5, username = $6, avatar_mediafile_id = $7, text = $8,
		    reply_to_comment_id = $9, is_team_reply = $10, created_at = $11
		WHERE id = $12
	`,
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
		SELECT id, post_union_id, platform, post_platform_id, user_platform_id, comment_platform_id, 
		       full_name, username, avatar_mediafile_id, text, reply_to_comment_id, is_team_reply, created_at
		FROM post_comment
		WHERE comment_platform_id = $1 AND platform = $2
	`, platformID, platform)

	var comment entity.Comment
	var avatarMediafileID *int

	if err := row.Scan(
		&comment.ID,
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
	); err != nil {
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

	// Получаем ID аватара, если он есть
	if comment.AvatarMediaFile != nil {
		avatarMediafileID = &comment.AvatarMediaFile.ID
	}

	row := tx.QueryRow(`
			INSERT INTO post_comment (post_union_id, platform, post_platform_id, user_platform_id, comment_platform_id, 
			                          full_name, username, avatar_mediafile_id, text, reply_to_comment_id, is_team_reply, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING id
		`,
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

func (c *Comment) GetComments(postUnionID int, offset time.Time, before bool, limit int) ([]*entity.Comment, error) {
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

	query := fmt.Sprintf(
		`
WITH all_comments AS (
 SELECT id, post_union_id, platform, post_platform_id, user_platform_id, comment_platform_id,
     full_name, username, avatar_mediafile_id, text, reply_to_comment_id, is_team_reply, created_at
 FROM post_comment
 WHERE ($1 = 0 OR post_union_id = $1)
 AND created_at %s $2
 ORDER BY created_at %s
 LIMIT $3
)
SELECT id, post_union_id, platform, post_platform_id, user_platform_id, comment_platform_id,
    full_name, username, avatar_mediafile_id, text, reply_to_comment_id, is_team_reply, created_at
FROM all_comments
ORDER BY CASE WHEN reply_to_comment_id IS NULL THEN 0 ELSE 1 END, created_at DESC
`, comparator, sortOrder,
	)
	rows, err := c.db.Queryx(query, postUnionID, offset, limit)
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
		SELECT id, post_union_id, platform, post_platform_id, user_platform_id, comment_platform_id, 
		       full_name, username, avatar_mediafile_id, text, reply_to_comment_id, is_team_reply, created_at
		FROM post_comment
		WHERE id = $1
	`, commentID)

	var comment entity.Comment
	var avatarMediafileID *int // Указатель для NULL значений

	// Извлекаем данные в структуру
	if err := row.Scan(
		&comment.ID,
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
