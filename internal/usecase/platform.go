package usecase

import "postic-backend/internal/entity"

type Platform interface {
	// AddPostInQueue добавляет пост в очередь на публикацию
	AddPostInQueue(postAction entity.PostAction) error
}
