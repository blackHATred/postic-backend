package entity

type Platform string

const (
	Telegram Platform = "tg"
	VK       Platform = "vk"
)

type VKChannel struct {
	ID      int
	UserID  int
	GroupID int
	APIKey  string
}
