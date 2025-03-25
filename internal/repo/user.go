package repo

import "postic-backend/internal/entity"

type User interface {
	// AddUser добавляет нового пользователя
	AddUser() (int, error)
	// GetTGChannel возвращает канал Телеграм как канал публикации пользователя
	GetTGChannel(userID int) (*entity.TGChannel, error)
	// GetVKChannel возвращает группу ВК как канал публикации пользователя
	GetVKChannel(userID int) (*entity.VKChannel, error)
	// PutVKChannel добавляет группу ВКонтакте как канал публикации для пользователя
	PutVKChannel(userID, groupID int, apiKey string) error
	// PutTGChannel добавляет канал Телеграм как канал публикации для пользователя
	PutTGChannel(userID, groupID int, apiKey string) error
}
