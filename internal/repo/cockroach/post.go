package cockroach

import (
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
)

type PostDB struct {
	db *sqlx.DB
}

func NewPost(db *sqlx.DB) repo.Post {
	return &PostDB{db: db}
}

func (p *PostDB) GetPostUnion(postUnionID int) (*entity.PostUnion, error) {
	var postUnion entity.PostUnion
	query := `
		SELECT id, user_id, team_id, text, platforms, created_at, pub_datetime
		FROM post_union
		WHERE id = $1
	`
	err := p.db.Get(&postUnion, query, postUnionID)
	if err != nil {
		return nil, err
	}

	// Получение прикрепленных медиафайлов
	var attachments []*entity.Upload
	attachmentQuery := `
		SELECT m.id, m.file_path, m.file_type, m.uploaded_by_user_id, m.created_at
		FROM post_union_mediafile pum
		JOIN mediafile m ON pum.mediafile_id = m.id
		WHERE pum.post_union_id = $1
	`
	err = p.db.Select(&attachments, attachmentQuery, postUnionID)
	if err != nil {
		return nil, err
	}
	postUnion.Attachments = attachments

	return &postUnion, nil
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
	//TODO implement me
	panic("implement me")
}

func (p *PostDB) GetScheduledPosts(status string) ([]*entity.ScheduledPost, error) {
	//TODO implement me
	panic("implement me")
}

func (p *PostDB) GetScheduledPost(postUnionID int) (*entity.ScheduledPost, error) {
	//TODO implement me
	panic("implement me")
}

func (p *PostDB) AddScheduledPost(scheduledPost *entity.ScheduledPost) (int, error) {
	//TODO implement me
	panic("implement me")
}

func (p *PostDB) EditScheduledPost(scheduledPost *entity.ScheduledPost) error {
	//TODO implement me
	panic("implement me")
}

func (p *PostDB) DeleteScheduledPost(postUnionID int) error {
	//TODO implement me
	panic("implement me")
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

func (p *PostDB) GetPostPlatform(postPlatformID int) (*entity.PostPlatform, error) {
	var postPlatform entity.PostPlatform
	query := `
		SELECT id, post_union_id, post_id, platform
		FROM post_platform
		WHERE id = $1
	`
	err := p.db.Get(&postPlatform, query, postPlatformID)
	if err != nil {
		return nil, err
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
	return postPlatformID, nil
}

func (p *PostDB) DeletePostPlatform() error {
	//TODO implement me
	panic("implement me")
}
