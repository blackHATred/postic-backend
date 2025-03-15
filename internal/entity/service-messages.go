package entity

type Message struct {
	Id int `json:"id"`
	// Type может быть "new", "update" или "delete"
	Type     string `json:"type"`
	Username string `json:"username"`
	Time     string `json:"time"`
	// Platform - платформа, с которой отправлен комментарий ("vk", "tg")
	Platform   string `json:"platform"`
	AvatarURL  string `json:"avatar_url"`
	Text       string `json:"text"`
	ReplyToUrl string `json:"reply_to_url"`
}

type ClientMessage struct {
	// VkKey
	VkKey string `json:"vk_key"`
	// VkGroupId
	VkGroupId int `json:"vk_group_id"`
	// TgChatId
	TgChatId int64 `json:"tg_chat_id"`
}
