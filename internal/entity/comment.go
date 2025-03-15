package entity

type Comment struct {
	// Platform указывает на платформу, на которой был опубликован комментарий
	Platform Platform `json:"platform"`
	// UserURL является ссылкой на профиль пользователя, который оставил комментарий
	UserURL string `json:"user_url"`
	// URL является ссылкой на комментарий
	URL string `json:"url"`
	// PostURL является ссылкой на пост, под которым оставлен комментарий
	PostURL string `json:"post_url"`

	Text        string   `json:"text"`
	PhotosURL   []string `json:"photos_url"`
	VideosURL   []string `json:"videos_url"`
	FilesURL    []string `json:"files_url"`
	GeoPosition string   `json:"geo_position"`
	AudiosURL   []string `json:"audios_url"`
}

type Reply struct {
}
