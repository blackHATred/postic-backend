package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
)

type Telegram struct {
	bot      *tgbotapi.BotAPI
	postRepo repo.Post
	userRepo repo.User
}

func NewTelegram() usecase.Post {
	return &Telegram{}
}

func (t Telegram) AddPost(request *entity.AddPostRequest) ([]int, error) {
	//TODO implement me
	panic("implement me")
}

func (t Telegram) EditPost(request *entity.EditPostRequest) ([]int, error) {
	//TODO implement me
	panic("implement me")
}

func (t Telegram) DeletePost(request *entity.DeletePostRequest) (int, error) {
	//TODO implement me
	panic("implement me")
}

func (t Telegram) GetPost(request *entity.GetPostRequest) (*entity.PostUnion, error) {
	//TODO implement me
	panic("implement me")
}

func (t Telegram) GetPosts(request *entity.GetPostsRequest) ([]*entity.PostUnion, error) {
	//TODO implement me
	panic("implement me")
}

func (t Telegram) GetPostStatus(request *entity.PostStatusRequest) ([]*entity.PostActionResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (t Telegram) DoAction(request *entity.DoActionRequest) ([]int, error) {
	//TODO implement me
	panic("implement me")
}
