package utils

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"postic-backend/internal/repo"
	"time"
)

type Auth interface {
	CheckAuth(tokenString string) (int, error)
	CheckAuthFromContext(c echo.Context) (int, error)
	Login(userId int) (string, error)
	CreateToken(userID int) (string, error)
}

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrInternal     = errors.New("internal error")
)

type jwtLoginClaims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

type AuthManager struct {
	jwtSecretKey  []byte
	userRepo      repo.User
	tokenLifetime time.Duration
}

func NewAuthManager(jwtSecretKey []byte, userRepo repo.User, tokenLifetime time.Duration) *AuthManager {
	return &AuthManager{
		jwtSecretKey:  jwtSecretKey,
		userRepo:      userRepo,
		tokenLifetime: tokenLifetime,
	}
}

// CheckAuth проверяет токен и возвращает ID пользователя, если токен валиден и юзер авторизован.
// Если пользователь не авторизован или токен невалиден, то возвращается ErrUnauthorized.
func (a *AuthManager) CheckAuth(tokenString string) (int, error) {
	claims := jwtLoginClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		return a.jwtSecretKey, nil
	})
	if err != nil {
		return -1, ErrUnauthorized
	}
	if !token.Valid {
		return -1, ErrUnauthorized
	}
	// todo: проверить существование пользователя
	return claims.UserID, nil
}

// CheckAuthFromContext проверяет токен и возвращает ID пользователя, если токен валиден и юзер авторизован.
// Если пользователь не авторизован или токен невалиден, то возвращается ErrUnauthorized.
func (a *AuthManager) CheckAuthFromContext(c echo.Context) (int, error) {
	cookie, err := c.Cookie("session")
	if err != nil {
		return -1, ErrUnauthorized
	}
	return a.CheckAuth(cookie.Value)
}

// Login авторизует пользователя и возвращает токен
func (a *AuthManager) Login(userId int) (string, error) {
	// потом должна быть полноценная авторизация
	user, err := a.userRepo.GetUser(userId)
	if err != nil {
		return "", errors.Join(ErrInternal, err)
	}
	return a.CreateToken(user.ID)
}

// CreateToken создает токен для пользователя
func (a *AuthManager) CreateToken(userID int) (string, error) {
	claims := jwtLoginClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.tokenLifetime)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.jwtSecretKey)
}
