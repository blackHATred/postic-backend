package entity

type LoginRequest struct {
	UserID int `json:"user_id"`
}

type User struct {
	ID           int    `json:"id" db:"id"`
	Name         string `json:"name" db:"name"`
	Email        string `json:"email" db:"email"`
	PasswordHash []byte `json:"-" db:"password_hash"`
}
