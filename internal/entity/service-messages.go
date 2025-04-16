package entity

type Message struct {
	Id int `json:"id"`
	// Type может быть "new", "update" или "delete"
	Type     string `json:"type"`
	PostId   int    `json:"post_id"`
	Username string `json:"username"`
	Time     string `json:"time"`
	// Platform - платформа, с которой отправлен комментарий ("vk", "tg")
	Platform string `json:"platform"`
	Text     string `json:"text"`
}
