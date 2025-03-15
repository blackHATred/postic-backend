package entity

type Summarize struct {
	// Markdown содержит сводку по комментариям с определенного поста
	Markdown string `json:"markdown"`
	// PostURL является ссылкой на пост, по которому производится суммарайз
	PostURL string `json:"post_url"`
}
