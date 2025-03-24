package entity

type LoginRequest struct {
	UserID int `json:"user_id"`
}

type SetVKRequest struct {
	GroupID int    `json:"group_id"`
	APIKey  string `json:"api_key"`
}
