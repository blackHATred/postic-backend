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

func (c *Comment) GetTGComments(postUnionID int, offset time.Time, limit int) ([]*entity.TelegramComment, error) {
	rows, err := c.db.Queryx(`
	SELECT id, post_tg_id, comment_id, user_id, text, created_at
	FROM post_tg_comment
	WHERE post_tg_id = (SELECT id FROM post_tg WHERE post_union_id = $1)
	AND created_at > $2
	ORDER BY created_at ASC
	LIMIT $3
`, postUnionID, offset, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []*entity.TelegramComment
	for rows.Next() {
		var comment entity.TelegramComment
		if err := rows.StructScan(&comment); err != nil {
			return nil, err
		}

		// Load user data
		var user entity.TelegramUser
		err = c.db.Get(&user, `SELECT user_id, username, first_name, last_name, photo_file_id FROM tg_user WHERE user_id = $1`, comment.UserID)
		if err != nil {
			return nil, err
		}
		comment.User = user

		// Load attachments
		var attachments []entity.TelegramMessageAttachment
		attachmentRows, err := c.db.Queryx(`
		SELECT id, comment_id, file_type, file_id
		FROM post_tg_comment_attachment
		WHERE comment_id = $1
	`, comment.ID)
		if err != nil {
			return nil, err
		}
		defer attachmentRows.Close()

		for attachmentRows.Next() {
			var attachment entity.TelegramMessageAttachment
			if err := attachmentRows.StructScan(&attachment); err != nil {
				return nil, err
			}
			attachments = append(attachments, attachment)
		}
		if err := attachmentRows.Err(); err != nil {
			return nil, err
		}
		comment.Attachments = attachments

		comments = append(comments, &comment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return comments, nil
}

func (c *Comment) AddTGComment(comment *entity.TelegramComment) (int, error) {
	// Обновляем или добавляем пользователя Telegram
	_, err := c.db.Exec(`
		INSERT INTO tg_user (user_id, username, first_name, last_name, photo_file_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE SET
			username = EXCLUDED.username,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			photo_file_id = EXCLUDED.photo_file_id
	`, comment.User.ID, comment.User.Username, comment.User.FirstName, comment.User.LastName, comment.User.PhotoFileID)
	if err != nil {
		return 0, err
	}

	var commentID int
	query := `
		INSERT INTO post_tg_comment (post_tg_id, comment_id, user_id, text, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	err = c.db.QueryRow(query, comment.PostTGID, comment.CommentID, comment.User.ID, comment.Text, comment.CreatedAt).Scan(&commentID)
	if err != nil {
		return 0, err
	}
	// теперь добавляем attachments, если такие имеются
	for _, attachment := range comment.Attachments {
		query = `
			INSERT INTO post_tg_comment_attachment (comment_id, file_type, file_id)
			VALUES ($1, $2, $3)
		`
		_, err = c.db.Exec(query, commentID, attachment.FileType, attachment.FileID)
		if err != nil {
			return 0, err
		}
	}
	return commentID, nil
}

func (c *Comment) GetTGAttachment(attachmentID int) (*entity.TelegramMessageAttachment, error) {
	var attachment entity.TelegramMessageAttachment
	query := `
		SELECT id, comment_id, file_type, file_id
		FROM post_tg_comment_attachment
		WHERE id = $1
	`
	err := c.db.Get(&attachment, query, attachmentID)
	if err != nil {
		return nil, err
	}
	return &attachment, nil
}
