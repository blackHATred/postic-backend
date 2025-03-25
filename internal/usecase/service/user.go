package service

import (
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
)

type User struct {
	userRepo repo.User
}

func NewUser(userRepo repo.User) usecase.User {
	return &User{userRepo: userRepo}
}

func (u *User) Register() (int, error) {
	return u.userRepo.AddUser()
}

func (u *User) Login(userID int) (int, error) {
	return userID, nil
}

func (u *User) SetVK(userID int, vkGroupID int, apiKey string) error {
	//TODO implement me
	panic("implement me")
}
