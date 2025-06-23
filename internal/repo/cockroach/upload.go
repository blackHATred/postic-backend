package cockroach

import (
	"context"
	"io"
	"net/http"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/minio/minio-go/v7"
)

type Upload struct {
	db          *sqlx.DB
	minioClient *minio.Client
}

func NewUpload(db *sqlx.DB, minioClient *minio.Client) (repo.Upload, error) {
	// Создаем бакет для user uploads, предварительно проверив, что его нет
	ctx := context.Background()
	exists, err := minioClient.BucketExists(ctx, "mediafiles")
	if err != nil {
		return nil, err
	}
	if !exists {
		err = minioClient.MakeBucket(ctx, "mediafiles", minio.MakeBucketOptions{
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
	upload := &entity.Upload{}
	query, args, err := sq.Select("*").From("mediafile").Where(sq.Eq{"id": id}).PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}
	err = u.db.Get(upload, query, args...)
	if err != nil {
		return nil, err
	}
	ctx := context.TODO()
	object, err := u.minioClient.GetObject(ctx, "mediafiles", upload.FilePath, minio.GetObjectOptions{Checksum: true})
	if err != nil {
		return nil, err
	}
	upload.RawBytes = object
	return upload, nil
}

func (u *Upload) GetUploadInfo(id int) (*entity.Upload, error) {
	upload := &entity.Upload{}
	query, args, err := sq.Select("*").From("mediafile").Where(sq.Eq{"id": id}).PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}
	err = u.db.Get(upload, query, args...)
	if err != nil {
		return nil, err
	}
	return upload, err
}

func (u *Upload) UploadFile(upload *entity.Upload) (int, error) {
	ctx := context.Background()
	rawBytes, err := io.ReadAll(upload.RawBytes)
	if err != nil {
		return 0, err
	}
	_, err = upload.RawBytes.Seek(0, io.SeekStart) // Сбросим указатель на начало, чтобы MinIO мог прочитать файл
	if err != nil {
		return 0, err
	}
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
	builder := sq.Insert("mediafile").Columns("file_path", "file_type")
	if upload.UserID == nil {
		builder = builder.Values(upload.FilePath, upload.FileType)
	} else {
		builder = builder.Columns("uploaded_by_user_id").Values(upload.FilePath, upload.FileType, upload.UserID)
	}
	builder = builder.Suffix("RETURNING id").PlaceholderFormat(sq.Dollar)
	query, qargs, err := builder.ToSql()
	if err != nil {
		return 0, err
	}
	var uploadID int
	err = u.db.QueryRow(query, qargs...).Scan(&uploadID)
	if err != nil {
		return 0, err
	}
	return uploadID, nil
}

func (u *Upload) DeleteUpload(id int) error {
	upload, err := u.GetUploadInfo(id)
	if err != nil {
		return err
	}
	ctx := context.Background()
	err = u.minioClient.RemoveObject(ctx, "mediafiles", upload.FilePath, minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}
	query, args, err := sq.Delete("mediafile").Where(sq.Eq{"id": id}).PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return err
	}
	_, err = u.db.Exec(query, args...)
	return err
}
