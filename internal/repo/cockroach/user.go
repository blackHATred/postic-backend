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
	query := `SELECT id, secret FROM "user" WHERE id = $1`
	err := u.db.Get(&user, query, userID)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (u *User) GetUserBySecret(secret string) (*entity.User, error) {
	var user entity.User
	query := `SELECT id, secret FROM "user" WHERE secret = $1`
	err := u.db.Get(&user, query, secret)
	if err != nil {
		return nil, err
	}
	return &user, nil
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

func (u *User) PutTGChannel(userID, channelID, discussionID int) error {
	query := `
		INSERT INTO channel_tg (user_id, channel_id, discussion_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		SET channel_id = EXCLUDED.channel_id,
		    discussion_id = EXCLUDED.discussion_id
	`
	_, err := u.db.Exec(query, userID, channelID, discussionID)
	return err
}
