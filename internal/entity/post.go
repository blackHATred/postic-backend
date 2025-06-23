package entity

import (
	"errors"
	"time"
	"unicode/utf8"
)

type GetPostRequest struct {
	UserID      int `query:"-"`
	TeamID      int `query:"team_id"`
	PostUnionID int `query:"post_union_id"`
}

type GetPostsRequest struct {
	UserID int        `query:"-"`
	TeamID int        `query:"team_id"`
	Offset *time.Time `query:"offset"`
	Before bool       `query:"before"`
	Limit  int        `query:"limit"`
	Filter *string    `query:"filter"`
}

type AddPostRequest struct {
	UserID      int        `json:"-"`
	TeamID      int        `json:"team_id"`
	Text        string     `json:"text"`
	PubDateTime *time.Time `json:"pub_datetime,omitempty"`
	Attachments []int      `json:"attachments"`
	Platforms   []string   `json:"platforms"`
}

func (r *AddPostRequest) IsValid() error {
	if r.Text == "" && len(r.Attachments) == 0 {
		return errors.New("text and attachments are empty")
	}
	if len(r.Platforms) == 0 {
		return errors.New("platforms are empty")
	}
	for _, platform := range r.Platforms {
		if platform == "tg" && len(r.Attachments) == 0 && utf8.RuneCountInString(r.Text) > 4096 {
			return errors.New("text is too long for telegram")
		}
		if platform == "tg" && len(r.Attachments) > 0 && utf8.RuneCountInString(r.Text) > 1024 {
			return errors.New("text is too long for telegram with attachments")
		}
		if platform == "vk" && utf8.RuneCountInString(r.Text) > 16384 {
			return errors.New("text is too long for vkontakte")
		}
		if platform == "fb" && utf8.RuneCountInString(r.Text) > 63206 {
			return errors.New("text is too long for facebook")
		}
		if platform == "ok" && utf8.RuneCountInString(r.Text) > 32000 {
			return errors.New("text is too long for odnoklassniki")
		}
		if platform == "ig" && utf8.RuneCountInString(r.Text) > 2200 {
			return errors.New("text is too long for instagram")
		}
	}
	return nil
}

type EditPostRequest struct {
	UserID      int
	TeamID      int    `json:"team_id"`
	PostUnionID int    `json:"post_union_id"`
	Text        string `json:"text"`
}

func (r *EditPostRequest) IsValid(platforms []string) error {
	for _, platform := range platforms {
		if platform == "tg" && utf8.RuneCountInString(r.Text) > 4096 {
			return errors.New("text is too long for telegram")
		}
		if platform == "vk" && utf8.RuneCountInString(r.Text) > 16384 {
			return errors.New("text is too long for vkontakte")
		}
		if platform == "fb" && utf8.RuneCountInString(r.Text) > 63206 {
			return errors.New("text is too long for facebook")
		}
		if platform == "ok" && utf8.RuneCountInString(r.Text) > 32000 {
			return errors.New("text is too long for odnoklassniki")
		}
		if platform == "ig" && utf8.RuneCountInString(r.Text) > 2200 {
			return errors.New("text is too long for instagram")
		}
	}
	return nil
}

type DeletePostRequest struct {
	UserID      int
	TeamID      int `json:"team_id"`
	PostUnionID int `json:"post_union_id"`
}

type PostUnion struct {
	ID          int        `json:"id" db:"id"`
	Text        string     `json:"text" db:"text"`
	Platforms   []string   `json:"platforms" db:"platforms"`
	PubDate     *time.Time `json:"pub_datetime" db:"pub_datetime"`
	Attachments []*Upload  `json:"attachments" db:"attachments"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UserID      int        `json:"user_id" db:"user_id"`
	TeamID      int        `json:"team_id" db:"team_id"`
}

type DoActionRequest struct {
	UserID      int    `json:"-"`
	TeamID      int    `json:"team_id"`
	PostUnionID int    `json:"post_union_id" db:"post_union_id"`
	Operation   string `json:"operation" db:"op"`
	Platform    string `json:"platform" db:"platform"`
}

type PostAction struct {
	ID          int       `db:"id"`
	PostUnionID *int      `db:"post_union_id"`
	Operation   string    `db:"op"`
	Platform    string    `db:"platform"`
	Status      string    `db:"status"`
	ErrMessage  string    `db:"error_message"`
	CreatedAt   time.Time `db:"created_at"`
}

type PostUnionList struct {
	Posts []*PostUnion `json:"posts"`
}

type TgPostPlatformGroup struct {
	TgPostID       int `db:"tg_post_id"`
	PostPlatformID int `db:"post_platform_id"`
}

type PostPlatform struct {
	ID          int    `db:"id"`
	PostUnionId int    `db:"post_union_id"`
	PostId      int    `db:"post_id"`
	Platform    string `db:"platform"`
	TGChannelID *int   `db:"tg_channel_id"` // ID канала в телеге
	VKChannelID *int   `db:"vk_channel_id"` // ID группы в ВК

	TgPostPlatformGroup []TgPostPlatformGroup // Есть только у Platform = tg
}

type PostStatusRequest struct {
	UserID      int `query:"-"`
	TeamID      int `query:"team_id"`
	PostUnionID int `query:"post_union_id"`
}

type PostActionResponse struct {
	PostID     int       `json:"post_id"`
	Operation  string    `json:"operation"`
	Platform   string    `json:"platform"`
	Status     string    `json:"status"`
	ErrMessage string    `json:"err_message"`
	CreatedAt  time.Time `json:"created_at"`
}

type ScheduledPost struct {
	PostUnionID int       `json:"post_union_id" db:"post_union_id"`
	ScheduledAt time.Time `json:"scheduled_at" db:"scheduled_at"`
	Status      string    `json:"status" db:"status"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// GeneratePostRequest запрос для генерации поста с помощью AI
type GeneratePostRequest struct {
	UserID int    `json:"-"`
	TeamID int    `json:"team_id"`
	Query  string `json:"query"`
}

// FixPostTextRequest запрос для исправления текста поста
type FixPostTextRequest struct {
	UserID int    `json:"-"`
	TeamID int    `json:"team_id"`
	Text   string `json:"text"`
}

// FixPostTextResponse ответ с исправленным текстом
type FixPostTextResponse struct {
	Text string `json:"text"`
}
