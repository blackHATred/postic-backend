package uploadservice

import (
	"bytes"
	"context"
	"errors"
	"io"
	uploadpb "postic-backend/internal/delivery/grpc/upload-service/proto"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
)

type UploadServiceServer struct {
	uploadpb.UnimplementedUploadServiceServer
	uploadRepo repo.Upload
}

func NewUploadServiceServer(uploadRepo repo.Upload) *UploadServiceServer {
	return &UploadServiceServer{
		uploadRepo: uploadRepo,
	}
}

func (s *UploadServiceServer) UploadFile(stream uploadpb.UploadService_UploadFileServer) error {
	var (
		buf      bytes.Buffer
		fileType string
		userID   *int
		fileName string
	)
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if chunk.FileType != "" {
			fileType = chunk.FileType
		}
		if chunk.UserId != 0 {
			uid := int(chunk.UserId)
			userID = &uid
		}
		if chunk.FileName != "" {
			fileName = chunk.FileName
		}
		buf.Write(chunk.Data)
	}
	upload := &entity.Upload{
		RawBytes: bytes.NewReader(buf.Bytes()),
		FileType: fileType,
		UserID:   userID,
		FilePath: fileName,
	}
	id, err := s.uploadRepo.UploadFile(upload)
	if err != nil {
		return err
	}
	return stream.SendAndClose(&uploadpb.UploadFileResponse{
		Id:       int64(id),
		FilePath: fileName,
	})
}

func (s *UploadServiceServer) DownloadChunk(ctx context.Context, req *uploadpb.DownloadChunkRequest) (*uploadpb.DownloadChunkResponse, error) {
	upload, err := s.uploadRepo.GetUpload(int(req.Id))
	if err != nil {
		return nil, err
	}
	if upload.RawBytes == nil {
		return nil, io.EOF
	}

	// Получаем размер файла сначала, чтобы не затрагивать seek операции позже
	size := upload.Size
	if size == 0 {
		// Попытаемся получить размер через Seeker
		if seeker, ok := upload.RawBytes.(io.Seeker); ok {
			cur, _ := seeker.Seek(0, io.SeekCurrent)
			sz, err := seeker.Seek(0, io.SeekEnd)
			if err == nil {
				size = sz
				seeker.Seek(cur, io.SeekStart) // возвращаем обратно
			}
		}
	}

	// Проверяем границы запроса
	if req.Offset < 0 || req.Length < 0 {
		return nil, errors.New("invalid offset or length")
	}
	if req.Offset >= size {
		return &uploadpb.DownloadChunkResponse{
			Data:      []byte{},
			Offset:    req.Offset,
			TotalSize: size,
		}, nil
	}

	// Корректируем длину если она выходит за границы файла
	maxLength := size - req.Offset
	if req.Length > maxLength {
		req.Length = maxLength
	}

	// Сдвигаем указатель на нужный offset
	_, err = upload.RawBytes.Seek(req.Offset, io.SeekStart)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, req.Length)
	n, err := upload.RawBytes.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return &uploadpb.DownloadChunkResponse{
		Data:      buf[:n],
		Offset:    req.Offset,
		TotalSize: size,
	}, nil
}

func (s *UploadServiceServer) GetUploadInfo(ctx context.Context, req *uploadpb.GetUploadInfoRequest) (*uploadpb.GetUploadInfoResponse, error) {
	upload, err := s.uploadRepo.GetUploadInfo(int(req.Id))
	if err != nil {
		return nil, err
	}
	resp := &uploadpb.GetUploadInfoResponse{
		Id:       int64(upload.ID),
		FilePath: upload.FilePath,
		FileType: upload.FileType,
		Size:     upload.Size,
	}
	if upload.UserID != nil {
		resp.UserId = int32(*upload.UserID)
	}
	if !upload.CreatedAt.IsZero() {
		resp.CreatedAt = upload.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return resp, nil
}

func (s *UploadServiceServer) DeleteUpload(ctx context.Context, req *uploadpb.DeleteUploadRequest) (*uploadpb.DeleteUploadResponse, error) {
	err := s.uploadRepo.DeleteUpload(int(req.Id))
	success := err == nil
	return &uploadpb.DeleteUploadResponse{Success: success}, err
}
