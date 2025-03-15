package repo

type Comment interface {
	GetComment
	AddComment() error
	EditComment() error
	DeleteComment() error
}

type GetComment interface {
	GetCommentByID() error
	GetCommentsByPost() error
}
