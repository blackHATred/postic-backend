package usecase

import (
	"errors"
	"postic-backend/internal/entity"
)

type PostPlatform interface {
	// AddPost ставит публикацию поста в очередь. Возвращает айди созданного action
	AddPost(request *entity.PostUnion) (int, error)
	// EditPost ставит в очередь задачу по редактированию поста. Возвращает айди созданного action
	EditPost(request *entity.EditPostRequest) (int, error)
	// DeletePost ставит в очередь задачу по удалению поста. Возвращает айди созданного action
	DeletePost(request *entity.DeletePostRequest) (int, error)
}

type PostUnion interface {
	// AddPostUnion создает PostUnion и ставит публикацию поста в очередь.
	// Возвращает айди созданного postUnion и айди созданных action
	AddPostUnion(request *entity.AddPostRequest) (int, []int, error)
	// EditPostUnion редактирует PostUnion. Возвращает айди созданных action
	EditPostUnion(request *entity.EditPostRequest) ([]int, error)
	// DeletePostUnion ставит в очередь задачу по удалению поста со всех платформ. Возвращает айди созданных action
	DeletePostUnion(request *entity.DeletePostRequest) ([]int, error)
	// GetPostUnion возвращает пост по ID
	GetPostUnion(request *entity.GetPostRequest) (*entity.PostUnion, error)
	// GetPosts возвращает список постов
	GetPosts(request *entity.GetPostsRequest) ([]*entity.PostUnion, error)
	// GetPostStatus возвращает статусы публикации поста по каждой из платформ
	GetPostStatus(request *entity.PostStatusRequest) ([]*entity.PostActionResponse, error)
	// DoAction добавляет операцию к PostUnion в очередь. Возвращает айди созданного action
	DoAction(request *entity.DoActionRequest) (int, error)
}

var (
	ErrPostUnavailableToEdit             = errors.New("пост недоступен для редактирования")
	ErrPostTextAndAttachmentsAreRequired = errors.New("пост должен содержать текст и/или вложения")
)
