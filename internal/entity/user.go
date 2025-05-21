package entity

import "time"

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type UpdateProfileRequest struct {
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
}

type User struct {
	ID               int       `json:"id" db:"id"`
	Nickname         string    `json:"nickname" db:"nickname"`
	Email            string    `json:"email" db:"email"`
	PasswordHash     string    `json:"-" db:"password_hash"`
	VkID             string    `json:"-" db:"vk_id"`
	VkAccessToken    string    `json:"-" db:"vk_access_token"`
	VkRefreshToken   string    `json:"-" db:"vk_refresh_token"`
	VkTokenExpiresAt time.Time `json:"-" db:"vk_token_expires_at"`
}

type UserProfile struct {
	ID       int    `json:"id"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
}
