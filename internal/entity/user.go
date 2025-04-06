package entity

type LoginRequest struct {
	UserID int `json:"user_id"`
}

type User struct {
	ID     int    `json:"id" db:"id"`
	Secret string `json:"secret" db:"secret"`
}
