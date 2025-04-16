package repo

import "postic-backend/internal/entity"

type Upload interface {
	// GetUpload возвращает загрузку по ID, включая файл
	GetUpload(id int) (*entity.Upload, error)
	// GetUploadInfo возвращает информацию о загрузке по ID, не включая файл
	GetUploadInfo(id int) (*entity.Upload, error)
	// UploadFile загружает файл
	UploadFile(upload *entity.Upload) (int, error)
}
