package cockroach

import (
	"bytes"
	"context"
	"github.com/jmoiron/sqlx"
	"github.com/minio/minio-go/v7"
	"io"
	"net/http"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
)

type Upload struct {
	db          *sqlx.DB
	minioClient *minio.Client
}

func NewUpload(db *sqlx.DB, minioClient *minio.Client) (repo.Upload, error) {
	// Создаем бакет для user uploads, предварительно проверив, что его нет
	ctx := context.TODO()
	exists, err := minioClient.BucketExists(ctx, "user-uploads")
	if err != nil {
		return nil, err
	}
	if !exists {
		err = minioClient.MakeBucket(ctx, "user-uploads", minio.MakeBucketOptions{
			Region:        "eu-central-1", // Предположим, что мы центральные европейцы
			ObjectLocking: true,
		})
		if err != nil {
			return nil, err
		}
	}
	return &Upload{
		db:          db,
		minioClient: minioClient,
	}, nil
}

func (u *Upload) GetUpload(id int) (*entity.Upload, error) {
	// Получаем upload из БД, потом загружаем его из S3
	upload := &entity.Upload{}
	query := `SELECT * FROM mediafile WHERE id = $1`
	err := u.db.Get(upload, query, id)
	if err != nil {
		return nil, err
	}
	ctx := context.TODO()
	object, err := u.minioClient.GetObject(ctx, "user-uploads", upload.FilePath, minio.GetObjectOptions{
		Checksum: true,
	})
	if err != nil {
		return nil, err
	}
	upload.RawBytes = object
	return upload, nil
}

func (u *Upload) GetUploadInfo(id int) (*entity.Upload, error) {
	upload := &entity.Upload{}
	query := `SELECT * FROM mediafile WHERE id = $1`
	err := u.db.Get(upload, query, id)
	if err != nil {
		return nil, err
	}
	return upload, err
}

func (u *Upload) UploadFile(upload *entity.Upload) (int, error) {
	// Добавляем файл в S3 хранилище и создаём запись в БД
	ctx := context.TODO()
	rawBytes, err := io.ReadAll(upload.RawBytes)
	if err != nil {
		return 0, err
	}
	// так как считали все байты, то нужно создать новый буфер - будем считать это допустимым оверхедом
	upload.RawBytes = bytes.NewBuffer(rawBytes)
	mediaType := http.DetectContentType(rawBytes)
	_, err = u.minioClient.PutObject(
		ctx,
		"mediafiles",
		upload.FilePath,
		upload.RawBytes,
		int64(len(rawBytes)),
		minio.PutObjectOptions{
			ContentType: mediaType,
		},
	)
	if err != nil {
		return 0, err
	}
	var uploadID int
	if upload.UserID == 0 {
		// Если загрузка не привязана к пользователю, то просто добавляем в БД
		query := `INSERT INTO mediafile (file_path, file_type) VALUES ($1, $2) RETURNING id`
		err = u.db.QueryRow(query, upload.FilePath, upload.FileType).Scan(&uploadID)
	} else {
		query := `INSERT INTO mediafile (file_path, file_type, uploaded_by_user_id) VALUES ($1, $2, $3) RETURNING id`
		err = u.db.QueryRow(query, upload.FilePath, upload.FileType, upload.UserID).Scan(&uploadID)
	}
	if err != nil {
		return 0, err
	}
	return uploadID, nil
}
