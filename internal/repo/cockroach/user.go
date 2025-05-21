package cockroach

import (
	"database/sql"
	"errors"
	"github.com/jmoiron/sqlx"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"time"
)

type User struct {
	db *sqlx.DB
}

func NewUser(db *sqlx.DB) repo.User {
	return &User{
		db: db,
	}
}

func (u *User) AddUser(user *entity.User) (int, error) {
	var userID int

	// Проверяем, существует ли пользователь с таким email
	if user.Email != "" {
		var exists bool
		query := `SELECT EXISTS(SELECT 1 FROM "user" WHERE email = $1)`
		err := u.db.QueryRow(query, user.Email).Scan(&exists)
		if err != nil {
			return 0, err
		}

		if exists {
			return 0, repo.ErrEmailExists
		}
	}

	query := `INSERT INTO "user" (nickname, email, password_hash) VALUES ($1, $2, $3) RETURNING id`
	err := u.db.QueryRow(query, user.Nickname, user.Email, user.PasswordHash).Scan(&userID)
	if err != nil {
		return 0, err
	}
	return userID, nil
}

func (u *User) GetUser(userID int) (*entity.User, error) {
	var user entity.User
	query := `SELECT id, nickname, email, password_hash, vk_id, vk_access_token, vk_refresh_token, vk_token_expires_at
			  FROM "user" WHERE id = $1`
	err := u.db.Get(&user, query, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repo.ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (u *User) GetUserByEmail(email string) (*entity.User, error) {
	var user entity.User
	query := `SELECT id, nickname, email, password_hash, vk_id, vk_access_token, vk_refresh_token, vk_token_expires_at
			  FROM "user" WHERE email = $1`
	err := u.db.Get(&user, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repo.ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (u *User) UpdatePassword(userID int, passwordHash string) error {
	query := `UPDATE "user" SET password_hash = $1 WHERE id = $2`
	result, err := u.db.Exec(query, passwordHash, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return repo.ErrUserNotFound
	}

	return nil
}

func (u *User) UpdateProfile(userID int, profile *entity.UpdateProfileRequest) error {
	// Проверяем, существует ли пользователь с таким email, кроме текущего
	if profile.Email != "" {
		var exists bool
		query := `SELECT EXISTS(SELECT 1 FROM "user" WHERE email = $1 AND id != $2)`
		err := u.db.QueryRow(query, profile.Email, userID).Scan(&exists)
		if err != nil {
			return err
		}

		if exists {
			return repo.ErrEmailExists
		}
	}

	query := `UPDATE "user" SET nickname = $1, email = $2 WHERE id = $3`
	result, err := u.db.Exec(query, profile.Nickname, profile.Email, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return repo.ErrUserNotFound
	}

	return nil
}

func (u *User) UpdateVkAuth(userID int, vkID, accessToken, refreshToken string, expiresAt int64) error {
	expiresTime := time.Unix(expiresAt, 0)

	query := `UPDATE "user" SET vk_id = $1, vk_access_token = $2, vk_refresh_token = $3, vk_token_expires_at = $4 WHERE id = $5`
	result, err := u.db.Exec(query, vkID, accessToken, refreshToken, expiresTime, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return repo.ErrUserNotFound
	}

	return nil
}
