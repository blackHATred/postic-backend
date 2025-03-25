package service

import (
	"errors"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
)

type Upload struct {
	uploadRepo repo.Upload
}

func NewUpload(uploadRepo repo.Upload) usecase.Upload {
	return &Upload{
		uploadRepo: uploadRepo,
	}
}

func (u Upload) UploadFile(upload *entity.Upload) (int, error) {
	return u.uploadRepo.UploadFile(upload)
}

func (u Upload) GetUpload(id int, userId int) (*entity.Upload, error) {
	upload, err := u.uploadRepo.GetUpload(id)
	if err != nil {
		return nil, err
	}
	if upload.UserID != userId {
		return nil, errors.New("file does not belong to this user")
	}
	return upload, nil
}
