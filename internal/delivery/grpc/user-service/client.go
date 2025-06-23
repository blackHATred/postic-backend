package userservice

import (
	"context"
	"fmt"
	"postic-backend/internal/entity"
	"postic-backend/internal/usecase"
	"time"

	userservice "postic-backend/internal/delivery/grpc/user-service/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// UserServiceClient реализует интерфейс usecase.User через gRPC
type UserServiceClient struct {
	client userservice.UserServiceClient
	conn   *grpc.ClientConn
}

// NewUserServiceClient создает новый gRPC клиент для user service
func NewUserServiceClient(address string) (usecase.User, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := userservice.NewUserServiceClient(conn)

	return &UserServiceClient{
		client: client,
		conn:   conn,
	}, nil
}

// Close закрывает соединение с gRPC сервером
func (c *UserServiceClient) Close() error {
	return c.conn.Close()
}

// Register регистрирует нового пользователя
func (c *UserServiceClient) Register(req *entity.RegisterRequest) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.Register(ctx, &userservice.RegisterRequest{
		Nickname: req.Nickname,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return 0, err
	}

	if resp.Error != "" {
		return 0, parseError(resp.Error)
	}

	return int(resp.UserId), nil
}

// Login авторизует пользователя
func (c *UserServiceClient) Login(email, password string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.Login(ctx, &userservice.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		return 0, err
	}

	if resp.Error != "" {
		return 0, parseError(resp.Error)
	}

	return int(resp.UserId), nil
}

// GetUser возвращает пользователя по его идентификатору
func (c *UserServiceClient) GetUser(userID int) (*entity.UserProfile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.GetUser(ctx, &userservice.GetUserRequest{
		UserId: int32(userID),
	})
	if err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, parseError(resp.Error)
	}

	return &entity.UserProfile{
		ID:       int(resp.Id),
		Nickname: resp.Nickname,
		Email:    resp.Email,
	}, nil
}

// UpdatePassword обновляет пароль пользователя
func (c *UserServiceClient) UpdatePassword(userID int, oldPassword, newPassword string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.UpdatePassword(ctx, &userservice.UpdatePasswordRequest{
		UserId:      int32(userID),
		OldPassword: oldPassword,
		NewPassword: newPassword,
	})
	if err != nil {
		return err
	}

	if resp.Error != "" {
		return parseError(resp.Error)
	}

	return nil
}

// UpdateProfile обновляет профиль пользователя
func (c *UserServiceClient) UpdateProfile(userID int, profile *entity.UpdateProfileRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.UpdateProfile(ctx, &userservice.UpdateProfileRequest{
		UserId:   int32(userID),
		Nickname: profile.Nickname,
		Email:    profile.Email,
	})
	if err != nil {
		return err
	}

	if resp.Error != "" {
		return parseError(resp.Error)
	}

	return nil
}

// GetVKAuthURL возвращает URL для авторизации через VK
func (c *UserServiceClient) GetVKAuthURL() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.GetVKAuthURL(ctx, &userservice.GetVKAuthURLRequest{})
	if err != nil {
		return ""
	}

	return resp.AuthUrl
}

// HandleVKCallback обрабатывает ответ от VK после авторизации
func (c *UserServiceClient) HandleVKCallback(code string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.HandleVKCallback(ctx, &userservice.HandleVKCallbackRequest{
		Code: code,
	})
	if err != nil {
		return 0, err
	}

	if resp.Error != "" {
		return 0, parseError(resp.Error)
	}

	return int(resp.UserId), nil
}

// parseError преобразует строку ошибки обратно в соответствующий тип ошибки
func parseError(errStr string) error {
	switch errStr {
	case usecase.ErrEmailAlreadyExists.Error():
		return usecase.ErrEmailAlreadyExists
	case usecase.ErrInvalidEmail.Error():
		return usecase.ErrInvalidEmail
	case usecase.ErrPasswordTooShort.Error():
		return usecase.ErrPasswordTooShort
	case usecase.ErrPasswordTooLong.Error():
		return usecase.ErrPasswordTooLong
	case usecase.ErrVKAuthFailed.Error():
		return usecase.ErrVKAuthFailed
	default:
		// Для неизвестных ошибок возвращаем обычную ошибку
		return fmt.Errorf("%s", errStr)
	}
}
