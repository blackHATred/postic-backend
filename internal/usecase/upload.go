package usecase

import "postic-backend/internal/entity"

type Upload interface {
	// UploadFile сохраняет файл в папку и возвращает его айди
	UploadFile(upload *entity.Upload) (int, error)
	// GetUpload возвращает файл по его айди. Если файл принадлежит другому пользователю, возвращает ошибку
	GetUpload(id int, userId int) (*entity.Upload, error)
}
