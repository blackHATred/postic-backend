package entity

import "time"

type Comment struct {
	// ID является уникальным идентификатором комментария
	ID int `json:"id"`
	// Platform указывает на платформу, на которой был опубликован комментарий
	Platform string `json:"platform"`
	// UserID является ID профиля пользователя, который оставил комментарий, с привязкой к платформе
	UserID int `json:"user_url"`
	// URL является ссылкой на комментарий
	URL string `json:"url"`
	// PostURL является ссылкой на пост, под которым оставлен комментарий
	PostURL string `json:"post_url"`

	Text      string   `json:"text"`
	PhotosURL []string `json:"photos_url"`
	VideosURL []string `json:"videos_url"`
	FilesURL  []string `json:"files_url"`
	AudiosURL []string `json:"audios_url"`
}

type WebSocketCommentRequest struct {
	Type        string                       `json:"type"`
	GetComments *WebSocketGetCommentsRequest `json:"get_comments"`
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
	// Text является текстом комментария
	Text string `json:"text" db:"text"`
	// CreatedAt является временем создания комментария
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	// Attachments является массивом вложений к комментарию
	Attachments []TelegramMessageAttachment `json:"attachments"`
}

type TelegramMessageAttachment struct {
	ID        int    `json:"id" db:"id"`
	CommentID int    `json:"comment_id" db:"comment_id"`
	FileType  string `json:"file_type" db:"file_type"`
	FileID    string `json:"file_id" db:"file_id"`
	RawBytes  []byte // Заполняется только если нужно получить содержимое файла
}
