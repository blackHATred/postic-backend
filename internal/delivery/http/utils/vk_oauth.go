package utils

import (
	"context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/vk"
)

// VKOAuth представляет конфигурацию OAuth для работы с ВКонтакте
type VKOAuth struct {
	config *oauth2.Config
}

// NewVKOAuth создает новый экземпляр VKOAuth
func NewVKOAuth(clientID, clientSecret, redirectURL string) *VKOAuth {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     vk.Endpoint,
		Scopes:       []string{"email", "offline"},
	}

	return &VKOAuth{
		config: config,
	}
}

// GetAuthURL возвращает URL для авторизации через VK
func (v *VKOAuth) GetAuthURL(state string) string {
	return v.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// Exchange обменивает код авторизации на токен
func (v *VKOAuth) Exchange(code string) (*oauth2.Token, error) {
	return v.config.Exchange(context.TODO(), code)
}
