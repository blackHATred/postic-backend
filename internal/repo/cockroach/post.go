package cockroach

import (
	"github.com/jmoiron/sqlx"
	"github.com/labstack/gommon/log"
	"github.com/lib/pq"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"strings"
)

type PostDB struct {
	db *sqlx.DB
}

func NewPost(db *sqlx.DB) repo.Post {
	return PostDB{db: db}
}

func (p PostDB) GetPostsByUserID(userID int) ([]*entity.PostUnion, error) {
	var posts []*entity.PostUnion
	query := `SELECT * FROM post_union WHERE user_id = $1`
	err := p.db.Select(&posts, query, userID)
	if err != nil {
		log.Error("GetPostsByUserID: ", err)
		return nil, err
	}
	return posts, nil
}

func (p PostDB) GetPostUnion(postUnionID int) (*entity.PostUnion, error) {
	var post entity.PostUnion
	var platforms []string
	query := `SELECT id, user_id, text, platforms, created_at, pub_datetime FROM post_union WHERE id = $1`
	err := p.db.QueryRowx(query, postUnionID).Scan(&post.ID, &post.UserID, &post.Text, pq.Array(&platforms), &post.CreatedAt, &post.PubDate)
	if err != nil {
		return nil, err
	}
	post.Platforms = platforms
	return &post, nil
}

func (p PostDB) GetPostUnions(userID int) ([]*entity.PostUnion, error) {
	var posts []*entity.PostUnion
	query := `SELECT id, user_id, text, platforms, created_at, pub_datetime FROM post_union WHERE user_id = $1`
	rows, err := p.db.Queryx(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var post entity.PostUnion
		var platforms []string
		if err := rows.Scan(&post.ID, &post.UserID, &post.Text, pq.Array(&platforms), &post.CreatedAt, &post.PubDate); err != nil {
			return nil, err
		}
		post.Platforms = platforms
		posts = append(posts, &post)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return posts, nil
}

func (p PostDB) AddPostUnion(union *entity.PostUnion) (int, error) {
	var postUnionID int
	platformsArray := "{" + strings.Join(union.Platforms, ",") + "}"
	query := `INSERT INTO post_union (user_id, text, platforms, created_at, pub_datetime) 
			  VALUES ($1, $2, $3, $4, $5) RETURNING id`
	err := p.db.QueryRow(query, union.UserID, union.Text, platformsArray, union.CreatedAt, union.PubDate).Scan(&postUnionID)
	if err != nil {
		return 0, err
	}
	return postUnionID, nil
}

func (p PostDB) GetPostAction(postUnionID int, platform string, last bool) (*entity.PostAction, error) {
	var action entity.PostAction
	query := `SELECT * FROM post_action WHERE post_union_id = $1 AND platform = $2 ORDER BY created_at DESC LIMIT 1`
	err := p.db.Get(&action, query, postUnionID, platform)
	if err != nil {
		return nil, err
	}
	return &action, nil
}

func (p PostDB) AddPostAction(action *entity.PostAction) (int, error) {
	var postActionID int
	query := `INSERT INTO post_action (post_union_id, platform, status, error_message, created_at) 
			  VALUES ($1, $2, $3, $4, $5) RETURNING id`
	err := p.db.QueryRow(query, action.PostUnionID, action.Platform, action.Status, action.ErrMessage, action.CreatedAt).Scan(&postActionID)
	if err != nil {
		return 0, err
	}
	return postActionID, nil
}

func (p PostDB) EditPostActionStatus(postUnionID int, status, errorMessage string) error {
	query := `UPDATE post_action SET status = $1, error_message = $2 WHERE post_union_id = $3`
	_, err := p.db.Exec(query, status, errorMessage, postUnionID)
	return err
}

func (p PostDB) AddPostVK(postUnionID, postID int) error {
	query := `INSERT INTO post_vk (post_union_id, post_id) VALUES ($1, $2)`
	_, err := p.db.Exec(query, postUnionID, postID)
	return err
}

func (p PostDB) AddPostTG(postUnionID, postID int) error {
	query := `INSERT INTO post_tg (post_union_id, post_id) VALUES ($1, $2)`
	_, err := p.db.Exec(query, postUnionID, postID)
	return err
}
