package repo

import "postic-backend/internal/entity"

type Upload interface {
	// GetUpload возвращает загрузку по ID
	GetUpload(id int) (*entity.Upload, error)
	// UploadFile загружает файл
	UploadFile(upload *entity.Upload) (int, error)
}
