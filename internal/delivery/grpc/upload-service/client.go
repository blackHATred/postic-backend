package uploadservice

import (
	"bytes"
	"context"
	"errors"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	uploadpb "postic-backend/internal/delivery/grpc/upload-service/proto"
)

type Client struct {
	conn   *grpc.ClientConn
	client uploadpb.UploadServiceClient
}

func NewClient(addr string) (*Client, error) {
	cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	client := uploadpb.NewUploadServiceClient(cc)
	return &Client{conn: cc, client: client}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// UploadFile отправляет файл чанками
func (c *Client) UploadFile(ctx context.Context, fileName, fileType string, userID int, r io.Reader) (*uploadpb.UploadFileResponse, error) {
	stream, err := c.client.UploadFile(ctx)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 1024*1024) // 1MB chunk
	for {
		n, err := r.Read(buf)
		if n > 0 {
			chunk := &uploadpb.UploadFileChunk{
				FileName: fileName,
				FileType: fileType,
				UserId:   int32(userID),
				Data:     buf[:n],
			}
			if err := stream.Send(chunk); err != nil {
				return nil, err
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return stream.CloseAndRecv()
}

// DownloadChunk читает кусок файла по offset/length
func (c *Client) DownloadChunk(ctx context.Context, id int64, offset, length int64) ([]byte, error) {
	resp, err := c.client.DownloadChunk(ctx, &uploadpb.DownloadChunkRequest{
		Id:     id,
		Offset: offset,
		Length: length,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetUploadInfo получает метаданные файла
func (c *Client) GetUploadInfo(ctx context.Context, id int64) (*uploadpb.GetUploadInfoResponse, error) {
	return c.client.GetUploadInfo(ctx, &uploadpb.GetUploadInfoRequest{Id: id})
}

// DeleteUpload удаляет файл
func (c *Client) DeleteUpload(ctx context.Context, id int64) (bool, error) {
	resp, err := c.client.DeleteUpload(ctx, &uploadpb.DeleteUploadRequest{Id: id})
	if err != nil {
		return false, err
	}
	return resp.Success, nil
}

// ReadSeeker реализация для удалённого файла через DownloadChunk
// Используется для интеграции с кодом, который требует io.ReadSeeker

type RemoteReadSeeker struct {
	client *Client
	ctx    context.Context
	fileID int64
	offset int64
	size   int64
	buf    *bytes.Reader
}

func NewRemoteReadSeeker(client *Client, ctx context.Context, fileID int64, size int64) *RemoteReadSeeker {
	return &RemoteReadSeeker{
		client: client,
		ctx:    ctx,
		fileID: fileID,
		size:   size,
		buf:    bytes.NewReader(nil),
	}
}

func (r *RemoteReadSeeker) Read(p []byte) (int, error) {
	if r.offset >= r.size {
		return 0, io.EOF
	}

	if r.buf.Len() == 0 {
		// Определяем размер чанка для загрузки (минимум 64KB или размер буфера)
		chunkSize := int64(len(p))
		if chunkSize < 65536 {
			chunkSize = 65536
		}

		// Убеждаемся, что не превысим размер файла
		remaining := r.size - r.offset
		if chunkSize > remaining {
			chunkSize = remaining
		}

		// Загрузить новый чанк
		data, err := r.client.DownloadChunk(r.ctx, r.fileID, r.offset, chunkSize)
		if err != nil {
			return 0, err
		}
		if len(data) == 0 {
			return 0, io.EOF
		}
		r.buf = bytes.NewReader(data)
	}

	n, err := r.buf.Read(p)
	r.offset += int64(n)
	return n, err
}

func (r *RemoteReadSeeker) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = r.offset + offset
	case io.SeekEnd:
		abs = r.size + offset
	default:
		return 0, errors.New("invalid seek")
	}
	if abs < 0 {
		return 0, errors.New("invalid seek: negative position")
	}
	if abs > r.size {
		abs = r.size
	}
	r.offset = abs
	r.buf = bytes.NewReader(nil) // сбросить буфер
	return abs, nil
}

// Size возвращает размер файла
func (r *RemoteReadSeeker) Size() int64 {
	return r.size
}
