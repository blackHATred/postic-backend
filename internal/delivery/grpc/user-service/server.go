package userservice

import (
	"context"
	userservice "postic-backend/internal/delivery/grpc/user-service/proto"
	"postic-backend/internal/entity"
	"postic-backend/internal/usecase"
)

// UserServiceImpl реализует gRPC сервер для пользователей
type UserServiceImpl struct {
	userservice.UnimplementedUserServiceServer
	userUseCase usecase.User
}

// NewUserServiceServer создает новый экземпляр UserServiceImpl
func NewUserServiceServer(userUseCase usecase.User) *UserServiceImpl {
	return &UserServiceImpl{
		userUseCase: userUseCase,
	}
}

// Register регистрирует нового пользователя
func (s *UserServiceImpl) Register(ctx context.Context, req *userservice.RegisterRequest) (*userservice.RegisterResponse, error) {
	registerReq := &entity.RegisterRequest{
		Nickname: req.Nickname,
		Email:    req.Email,
		Password: req.Password,
	}

	userID, err := s.userUseCase.Register(registerReq)
	if err != nil {
		return &userservice.RegisterResponse{
			UserId: 0,
			Error:  err.Error(),
		}, nil
	}

	return &userservice.RegisterResponse{
		UserId: int32(userID),
		Error:  "",
	}, nil
}

// Login авторизует пользователя
func (s *UserServiceImpl) Login(ctx context.Context, req *userservice.LoginRequest) (*userservice.LoginResponse, error) {
	userID, err := s.userUseCase.Login(req.Email, req.Password)
	if err != nil {
		return &userservice.LoginResponse{
			UserId: 0,
			Error:  err.Error(),
		}, nil
	}

	return &userservice.LoginResponse{
		UserId: int32(userID),
		Error:  "",
	}, nil
}

// GetUser возвращает информацию о пользователе
func (s *UserServiceImpl) GetUser(ctx context.Context, req *userservice.GetUserRequest) (*userservice.GetUserResponse, error) {
	user, err := s.userUseCase.GetUser(int(req.UserId))
	if err != nil {
		return &userservice.GetUserResponse{
			Error: err.Error(),
		}, nil
	}

	return &userservice.GetUserResponse{
		Id:        int32(user.ID),
		Nickname:  user.Nickname,
		Email:     user.Email,
		CreatedAt: "", // Пока оставим пустым, так как нет полей в entity.UserProfile
		UpdatedAt: "", // Пока оставим пустым, так как нет полей в entity.UserProfile
		Error:     "",
	}, nil
}

// UpdatePassword обновляет пароль пользователя
func (s *UserServiceImpl) UpdatePassword(ctx context.Context, req *userservice.UpdatePasswordRequest) (*userservice.UpdatePasswordResponse, error) {
	err := s.userUseCase.UpdatePassword(int(req.UserId), req.OldPassword, req.NewPassword)
	if err != nil {
		return &userservice.UpdatePasswordResponse{
			Error: err.Error(),
		}, nil
	}

	return &userservice.UpdatePasswordResponse{
		Error: "",
	}, nil
}

// UpdateProfile обновляет профиль пользователя
func (s *UserServiceImpl) UpdateProfile(ctx context.Context, req *userservice.UpdateProfileRequest) (*userservice.UpdateProfileResponse, error) {
	updateReq := &entity.UpdateProfileRequest{
		Nickname: req.Nickname,
		Email:    req.Email,
	}

	err := s.userUseCase.UpdateProfile(int(req.UserId), updateReq)
	if err != nil {
		return &userservice.UpdateProfileResponse{
			Error: err.Error(),
		}, nil
	}

	return &userservice.UpdateProfileResponse{
		Error: "",
	}, nil
}

// GetVKAuthURL возвращает URL для авторизации через VK
func (s *UserServiceImpl) GetVKAuthURL(ctx context.Context, req *userservice.GetVKAuthURLRequest) (*userservice.GetVKAuthURLResponse, error) {
	authURL := s.userUseCase.GetVKAuthURL()
	return &userservice.GetVKAuthURLResponse{
		AuthUrl: authURL,
		Error:   "",
	}, nil
}

// HandleVKCallback обрабатывает ответ от VK после авторизации
func (s *UserServiceImpl) HandleVKCallback(ctx context.Context, req *userservice.HandleVKCallbackRequest) (*userservice.HandleVKCallbackResponse, error) {
	userID, err := s.userUseCase.HandleVKCallback(req.Code)
	if err != nil {
		return &userservice.HandleVKCallbackResponse{
			UserId: 0,
			Error:  err.Error(),
		}, nil
	}

	return &userservice.HandleVKCallbackResponse{
		UserId: int32(userID),
		Error:  "",
	}, nil
}
