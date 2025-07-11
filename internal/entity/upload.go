package entity

import (
	"io"
	"time"
)

type Upload struct {
	ID        int           `json:"id" db:"id"`
	RawBytes  io.ReadSeeker `json:"-"`
	FilePath  string        `json:"file_path" db:"file_path"`
	FileType  string        `json:"file_type" db:"file_type"`
	UserID    *int          `json:"uploaded_by_user_id,omitempty" db:"uploaded_by_user_id"`
	CreatedAt time.Time     `json:"created_at" db:"created_at"`
	Size      int64         `json:"size" db:"-"`
}
