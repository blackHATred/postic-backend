package utils

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"strconv"
	"time"
)

type CookieManager struct {
	secureCookies bool
}

func NewCookieManager(secureCookies bool) *CookieManager {
	return &CookieManager{secureCookies: secureCookies}
}

func (c *CookieManager) NewUserIDCookie(userID int, expires time.Time) *http.Cookie {
	cookie := http.Cookie{
		Name:     "session",
		Value:    strconv.Itoa(userID),
		Expires:  expires,
		HttpOnly: true,
		Secure:   c.secureCookies,
		Path:     "/",
	}
	return &cookie
}

func (c *CookieManager) GetUserIDFromContext(ctx echo.Context) (int, error) {
	// Потом нужно будет сделать нормальную проверку на авторизацию
	cookie, err := ctx.Cookie("user_id")
	if err != nil {
		return -1, err
	}
	userID, err := strconv.Atoi(cookie.Value)
	if err != nil {
		return -1, err
	}
	return userID, nil
}
