package cockroach

import (
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
	//TODO implement me
	panic("implement me")
}

func (p PostDB) GetPostUnion(postUnionID int) (*entity.PostUnion, error) {
	//TODO implement me
	panic("implement me")
}

func (p PostDB) GetPostActionVK(postUnionID int) (*entity.PostActionVK, error) {
	//TODO implement me
	panic("implement me")
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
