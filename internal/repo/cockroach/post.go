package cockroach

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
)

type PostDB struct {
	DB *sqlx.DB
}

func NewPostRepo(db *sqlx.DB) repo.Post {
	return PostDB{DB: db}
}

func (p PostDB) PutChannel(userID, groupID int, apiKey string) error {
	//TODO implement me
	panic("implement me")
}

func (p PostDB) GetVKChannel(userID int) (*entity.VKChannel, error) {
	//TODO implement me
	panic("implement me")
}

func (p PostDB) AddPostUnion(union *entity.PostUnion) (int, error) {
	// тут нет teamID
	query := `INSERT INTO post_union (user_id, text, created_at, pud_datetime) 
	          VALUES ($1, $2, $3, $4, $5) RETURNING id`
	var postID int
	err := p.DB.QueryRow(query, union.UserID, union.Text, union.CreatedAt, union.PubDate).Scan(&postID)
	if err != nil {
		return 0, err
	}
	return postID, nil
}

func (p PostDB) GetPostUnion(postUnionID int) (*entity.PostUnion, error) {
	var post entity.PostUnion
	query := `SELECT id, user_id, text, created_at, pud_datetime FROM post_union WHERE id = $1`
	err := p.DB.Get(&post, query, postUnionID)
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func (p PostDB) GetPostActionVK(postUnionID int) (*entity.PostActionVK, error) {
	var action entity.PostActionVK
	query := `SELECT id, post_union_id FROM action_post_vk WHERE post_union_id = $1`
	err := p.DB.Get(&action, query, postUnionID)
	if err != nil {
		return nil, err
	}
	return &action, nil
}

func (p PostDB) AddPostActionVK(postUnionID int) (int, error) {
	//TODO implement me
	panic("implement me")
}

func (p PostDB) EditPostActionVK(postUnionID int, status, errorMessage string) error {
	//TODO implement me
	panic("implement me")
}

func (p PostDB) AddPostVK(postUnionID, postID int) error {
	//TODO implement me
	panic("implement me")
}

func (p PostDB) GetPosts(userID int) ([]*entity.PostUnion, error) {
	var posts []*entity.PostUnion
	query := `SELECT id, user_id, text, created_at, pud_datetime FROM post_union WHERE user_id = $1`
	rows, err := p.DB.Queryx(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var post entity.PostUnion
		err := rows.Scan(&post.ID, &post.UserID, &post.Text, &post.CreatedAt, &post.PubDate)
		if err != nil {
			return nil, err
		}
		posts = append(posts, &post)
	}

	return posts, nil
}

func (p PostDB) GetPostStatusVKTG(postID int) (*entity.GetPostStatusResponse, error) {
	var statusVk, statusTg, errMsg string
	query := `SELECT status, coalesce(error_message, '') FROM action_post_vk WHERE post_union_id = $1`
	err := p.DB.QueryRow(query, postID).Scan(&statusVk, &errMsg)
	if err != nil {
		return nil, fmt.Errorf("Ошибка получения статуса вк: %v", err)
	}

	// TODO: убрать после добавления телеграма
	statusTg = statusVk

	resp := &entity.GetPostStatusResponse{
		PostID:     postID,
		StatusVK:   statusVk,
		StatusTG:   statusTg,
		ErrMessage: errMsg,
	}
	return resp, nil
}
