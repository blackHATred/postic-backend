package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	uploadgrpc "postic-backend/internal/delivery/grpc/upload-service"
	"postic-backend/internal/entity"
	"postic-backend/internal/usecase"
	"strings"
)

type Upload struct {
	uploadClient *uploadgrpc.Client
}

func NewUpload(uploadClient *uploadgrpc.Client) usecase.Upload {
	return &Upload{
		uploadClient: uploadClient,
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
	resp, err := u.uploadClient.UploadFile(context.Background(), upload.FilePath, upload.FileType, derefInt(upload.UserID), upload.RawBytes)
	if err != nil {
		return 0, err
	}
	return int(resp.Id), nil
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
	info, err := u.uploadClient.GetUploadInfo(context.Background(), int64(id))
	if err != nil {
		return nil, err
	}
	// Получаем размер файла
	size := info.Size
	reader := uploadgrpc.NewRemoteReadSeeker(u.uploadClient, context.Background(), int64(id), size)
	return &entity.Upload{
		ID:        int(info.Id),
		FilePath:  info.FilePath,
		FileType:  info.FileType,
		UserID:    intPtr(int(info.UserId)),
		CreatedAt: parseTime(info.CreatedAt),
		RawBytes:  reader,
	}, nil
}

func derefInt(ptr *int) int {
	if ptr != nil {
		return *ptr
	}
	return 0
}

func intPtr(i int) *int {
	return &i
}

func parseTime(s string) time.Time {
	t, _ := time.Parse("2006-01-02T15:04:05Z07:00", s)
	return t
}
