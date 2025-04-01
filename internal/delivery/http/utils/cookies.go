package utils

import (
	"net/http"
	"time"
)

type Cookie interface {
	SetSessionCookie(token string, expires time.Time) *http.Cookie
}

type CookieManager struct {
	secureCookies bool
}

func NewCookieManager(secureCookies bool) Cookie {
	return &CookieManager{secureCookies: secureCookies}
}

func (c *CookieManager) SetSessionCookie(token string, expires time.Time) *http.Cookie {
	cookie := http.Cookie{
		Name:     "session",
		Value:    token,
		Expires:  expires,
		HttpOnly: true,
		Secure:   c.secureCookies,
		Path:     "/",
	}
	return &cookie
}
