package entity

import "time"

type Upload struct {
	ID        int `db:"id"`
	RawBytes  []byte
	FilePath  string    `db:"file_path"`
	FileType  string    `db:"file_type"`
	UserID    int       `db:"uploaded_by_user_id"`
	CreatedAt time.Time `db:"created_at"`
}
