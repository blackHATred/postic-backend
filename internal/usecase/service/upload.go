package service

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"strings"
)

type Upload struct {
	uploadRepo repo.Upload
}

func NewUpload(uploadRepo repo.Upload) usecase.Upload {
	return &Upload{
		uploadRepo: uploadRepo,
	}
}

func (u *Upload) UploadFile(upload *entity.Upload) (int, error) {
	// переводим название файла в base64 (без учета расширения файла) и добавляем к нему префикс uuid,
	// чтобы избежать проблем с кириллицей и пробелами
	strings.LastIndex(upload.FilePath, ".")
	upload.FilePath = fmt.Sprintf(
		"%s_%s.%s",
		uuid.New().String(),
		base64.StdEncoding.EncodeToString([]byte(upload.FilePath)),
		upload.FilePath[strings.LastIndex(upload.FilePath, ".")+1:],
	)
	return u.uploadRepo.UploadFile(upload)
}

func (u *Upload) GetUpload(id int, userId int) (*entity.Upload, error) {
	upload, err := u.uploadRepo.GetUpload(id)
	if err != nil {
		return nil, err
	}
	if upload.UserID != userId {
		return nil, errors.New("file does not belong to this user")
	}
	return upload, nil
}
