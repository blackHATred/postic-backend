package usecase

import (
	"postic-backend/internal/entity"
)

type Comment interface {
	GetLastComments(postUnionID int, limit int) ([]*entity.JustTextComment, error)
	GetSummarize(postUnionID int) (*entity.Summarize, error)
}
