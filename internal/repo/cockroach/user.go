package cockroach

import (
	"github.com/jmoiron/sqlx"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
)

type User struct {
	db *sqlx.DB
}

func NewUser(db *sqlx.DB) repo.User {
	return &User{
		db: db,
	}
}

func (u *User) AddUser() (int, error) {
	var userID int
	query := `INSERT INTO "user" DEFAULT VALUES RETURNING id`
	err := u.db.QueryRow(query).Scan(&userID)
	if err != nil {
		return 0, err
	}
	return userID, nil
}

func (u *User) GetUser(userID int) (*entity.User, error) {
	var user entity.User
	query := `SELECT id FROM "user" WHERE id = $1`
	err := u.db.Get(&user, query, userID)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
