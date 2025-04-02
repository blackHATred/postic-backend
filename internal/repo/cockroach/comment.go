package cockroach

import (
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
	row := tx.QueryRow(`
			INSERT INTO post_comment (post_union_id, platform, post_platform_id, user_platform_id, comment_platform_id, full_name, username, avatar_mediafile_id, text, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING id
		`,
		comment.PostUnionID,
		comment.Platform,
		comment.PostPlatformID,
		comment.UserPlatformID,
		comment.CommentPlatformID,
		comment.FullName,
		comment.Username,
		comment.AvatarMediaFileID,
		comment.Text,
		comment.CreatedAt,
	)
	err = row.Scan(&commentID)
	if err != nil {
		return 0, err
	}

	if len(comment.Attachments) > 0 {
		for _, attachment := range comment.Attachments {
			_, err := tx.Exec(`
				INSERT INTO post_comment_attachment (comment_id, mediafile_id)
				VALUES ($1, $2)
			`, commentID, attachment)
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

func (c *Comment) GetComments(postUnionID int, offset time.Time, limit int) ([]*entity.Comment, error) {
	rows, err := c.db.Queryx(`
		SELECT id, post_union_id, platform, post_platform_id, user_platform_id, comment_platform_id, full_name, username, avatar_mediafile_id, text, created_at
		FROM post_comment
		WHERE post_union_id = $1 AND created_at < $2
		ORDER BY created_at DESC
		LIMIT $3
	`, postUnionID, offset, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var comments []*entity.Comment
	for rows.Next() {
		var comment entity.Comment
		if err := rows.StructScan(&comment); err != nil {
			return nil, err
		}
		comment.Attachments = make([]int, 0)
		attachmentsRows, err := c.db.Queryx(
			`SELECT mediafile_id FROM post_comment_attachment WHERE comment_id = $1`,
			comment.ID,
		)
		if err != nil {
			return nil, err
		}
		for attachmentsRows.Next() {
			var attachmentID int
			if err := attachmentsRows.Scan(&attachmentID); err != nil {
				_ = attachmentsRows.Close()
				return nil, err
			}
			comment.Attachments = append(comment.Attachments, attachmentID)
		}
		_ = attachmentsRows.Close()
		comments = append(comments, &comment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return comments, nil
}
