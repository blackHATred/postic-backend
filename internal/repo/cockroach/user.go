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

func (u *User) GetTGChannel(userID int) (*entity.TGChannel, error) {
	var channel entity.TGChannel
	query := `SELECT id, user_id, channel_id, discussion_id FROM channel_tg WHERE user_id = $1`
	err := u.db.Get(&channel, query, userID)
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

func (u *User) GetVKChannel(userID int) (*entity.VKChannel, error) {
	//TODO implement me
	panic("implement me")
}

func (u *User) PutVKChannel(userID, groupID int, apiKey string) error {
	//TODO implement me
	panic("implement me")
}

func (u *User) PutTGChannel(userID, groupID int, apiKey string) error {
	//TODO implement me
	panic("implement me")
}
