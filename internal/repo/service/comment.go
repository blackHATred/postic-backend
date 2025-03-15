package service

import "postic-backend/internal/repo"

type Comment struct {
}

func NewComment() repo.Comment {
	return &Comment{}
}

func (c Comment) GetCommentByID() error {
	//TODO implement me
	panic("implement me")
}

func (c Comment) GetCommentsByPost() error {
	//TODO implement me
	panic("implement me")
}

func (c Comment) AddComment() error {
	//TODO implement me
	panic("implement me")
}

func (c Comment) EditComment() error {
	//TODO implement me
	panic("implement me")
}

func (c Comment) DeleteComment() error {
	//TODO implement me
	panic("implement me")
}
