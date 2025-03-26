package entity

import "time"

type WebSocketCommentRequest struct {
	Type        string                       `json:"type"`
	GetComments *WebSocketGetCommentsRequest `json:"get_comments,omitempty"`
}

type WebSocketGetCommentsRequest struct {
	PostUnionID int       `json:"post_union_id"`
	Offset      time.Time `json:"offset"`
}

type TelegramComment struct {
	// ID является уникальным идентификатором комментария
	ID int `json:"id" db:"id"`
	// PostTGID является уникальным идентификатором поста в Telegram, под которым был оставлен комментарий
	PostTGID int `json:"post_tg_id" db:"post_tg_id"`
	// CommentID является уникальным идентификатором комментария в Telegram
	CommentID int `json:"comment_id" db:"comment_id"`
	// UserID является ID профиля пользователя, который оставил комментарий
	UserID int `json:"user_id" db:"user_id"`
	// User является пользователем, который оставил комментарий
	User TelegramUser `json:"user"`
	// Text является текстом комментария
	Text string `json:"text" db:"text"`
	// CreatedAt является временем создания комментария
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	// Attachments является массивом вложений к комментарию
	Attachments []TelegramMessageAttachment `json:"attachments"`
}

type TelegramUser struct {
	// ID является уникальным идентификатором пользователя в Telegram
	ID int `json:"user_id" db:"user_id"`
	// Username является никнеймом пользователя в Telegram
	Username string `json:"username" db:"username"`
	// FirstName является именем пользователя в Telegram
	FirstName string `json:"first_name" db:"first_name"`
	// LastName является фамилией пользователя в Telegram
	LastName    string `json:"last_name" db:"last_name"`
	PhotoFileID string `json:"photo_file_id" db:"photo_file_id"`
}

type TelegramMessageAttachment struct {
	ID        int    `json:"id" db:"id"`
	CommentID int    `json:"comment_id" db:"comment_id"`
	FileType  string `json:"file_type" db:"file_type"`
	FileID    string `json:"file_id" db:"file_id"`
	RawBytes  []byte `json:"-"` // Заполняется только если нужно получить содержимое файла
}

type JustTextComment struct {
	Text string `json:"text" db:"text"`
}
