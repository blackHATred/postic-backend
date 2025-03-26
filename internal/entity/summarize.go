package entity

type Summarize struct {
	// Markdown содержит сводку по комментариям с определенного поста
	Markdown string `json:"markdown"`
	// PostUnionID является уникальным идентификатором поста
	PostUnionID string `json:"post_union_id"`
}
