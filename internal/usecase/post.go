package usecase

import "postic-backend/internal/entity"

type Post interface {
	// AddVKChannel добавляет группу ВКонтакте как канал публикации для пользователя
	AddVKChannel(userID int, groupID int, apiKey string) error
	// AddPost добавляет агрегированный пост
	AddPost(request *entity.AddPostRequest) error
}
