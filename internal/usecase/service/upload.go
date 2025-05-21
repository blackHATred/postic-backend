package service

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
	"net/http"
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
	// Проверка типа файла по расширению
	fileExt := strings.ToLower(upload.FilePath[strings.LastIndex(upload.FilePath, ".")+1:])

	// Проверяем соответствие типа файла и расширения
	switch upload.FileType {
	case "photo":
		if fileExt != "jpg" && fileExt != "jpeg" && fileExt != "png" {
			return 0, errors.New("неподдерживаемое расширение фото: допустимы только jpg, jpeg, png")
		}
	case "video":
		if fileExt != "mp4" {
			return 0, errors.New("неподдерживаемое расширение видео: допустимо только mp4")
		}
	default:
		return 0, errors.New("неподдерживаемый тип файла: допустимы только photo и video")
	}

	// Проверка MIME-типа на основе содержимого
	if err := validateMimeType(upload); err != nil {
		return 0, err
	}

	// переводим название файла в base64 (без учета расширения файла) и добавляем к нему префикс uuid,
	// чтобы избежать проблем с юникодом
	upload.FilePath = fmt.Sprintf(
		"%s_%s.%s",
		uuid.New().String(),
		base64.StdEncoding.EncodeToString([]byte(upload.FilePath)),
		fileExt,
	)
	return u.uploadRepo.UploadFile(upload)
}

// validateMimeType проверяет MIME-тип файла на основе его содержимого
func validateMimeType(upload *entity.Upload) error {
	// Сохраняем текущую позицию в файле
	currentPos, err := upload.RawBytes.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("ошибка чтения файла: %v", err)
	}
	defer func() { _, _ = upload.RawBytes.Seek(currentPos, io.SeekStart) }() // Возвращаем указатель на прежнюю позицию

	// Перемещаем указатель в начало файла для чтения заголовка
	_, _ = upload.RawBytes.Seek(0, io.SeekStart)

	// Читаем первые 512 байт для определения MIME-типа
	buffer := make([]byte, 512)
	n, err := upload.RawBytes.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("ошибка чтения файла: %v", err)
	}
	buffer = buffer[:n]

	// Определяем MIME-тип файла
	mimeType := http.DetectContentType(buffer)

	// Проверяем соответствие MIME-типа заявленному типу файла
	switch upload.FileType {
	case "photo":
		if !strings.HasPrefix(mimeType, "image/jpeg") && !strings.HasPrefix(mimeType, "image/png") {
			return fmt.Errorf("содержимое не соответствует формату фото: обнаружен MIME-тип %s", mimeType)
		}
	case "video":
		if !strings.HasPrefix(mimeType, "video/mp4") {
			return fmt.Errorf("содержимое не соответствует формату видео: обнаружен MIME-тип %s", mimeType)
		}
	}

	return nil
}

func (u *Upload) GetUpload(id int) (*entity.Upload, error) {
	upload, err := u.uploadRepo.GetUpload(id)
	if err != nil {
		return nil, err
	}
	return upload, nil
}
