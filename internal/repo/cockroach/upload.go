package cockroach

import (
	"github.com/jmoiron/sqlx"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
)

type Upload struct {
	db *sqlx.DB
}

func NewUpload(db *sqlx.DB) repo.Upload {
	return &Upload{
		db: db,
	}
}

func (u Upload) GetUpload(id int) (*entity.Upload, error) {
	//TODO implement me
	panic("implement me")
}
