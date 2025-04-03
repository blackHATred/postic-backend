package usecase

import "postic-backend/internal/entity"

type Post interface {
	// AddPost ставит публикацию поста в очередь. Возвращает айди созданных action
	AddPost(request *entity.AddPostRequest) ([]int, error)
	// EditPost ставит в очередь задачу по редактированию поста. Возвращает айди созданных action
	EditPost(request *entity.EditPostRequest) ([]int, error)
	// DeletePost ставит в очередь задачу по удалению поста со всех платформ. Возвращает айди созданного action
	DeletePost(request *entity.DeletePostRequest) (int, error)
	// GetPost возвращает пост по ID
	GetPost(request *entity.GetPostRequest) (*entity.PostUnion, error)
	// GetPosts возвращает список постов
	GetPosts(request *entity.GetPostsRequest) ([]*entity.PostUnion, error)
	// GetPostStatus возвращает статусы публикации поста по каждой из платформ
	GetPostStatus(request *entity.PostStatusRequest) ([]*entity.PostActionResponse, error)
	// DoAction добавляет операцию к PostUnion в очередь. Возвращает айди созданных action
	DoAction(request *entity.DoActionRequest) ([]int, error)
}
