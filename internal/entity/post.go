package entity

import (
	"errors"
	"time"
)

type GetPostRequest struct {
	UserID      int
	TeamID      int
	PostUnionID int `json:"post_union_id"`
}

type GetPostsRequest struct {
	UserID int
	TeamID int
	Offset int `json:"offset"`
	Limit  int `json:"-"`
}

type AddPostRequest struct {
	UserID int
	TeamID int
	Text   string `json:"text"`
	// PubDateTime указывается в UNIX timestamp UTC +0
	PubDateTime int      `json:"pub_datetime"`
	Attachments []int    `json:"attachments"`
	Platforms   []string `json:"platforms"`
}

func (r *AddPostRequest) IsValid() error {
	if r.Text == "" && len(r.Attachments) == 0 {
		return errors.New("text and attachments are empty")
	}
	// Запас в 5 минут сделан намеренно с целью предотвратить возможные издержки.
	// С точки зрения usecase добавлять пост в очередь на публикацию в прошлом или раннее чем через 5 минут
	// не имеет смысла
	if int64(r.PubDateTime) < time.Now().Add(5*time.Minute).Unix() {
		return errors.New("pub_datetime must be in the future")
	}
	if len(r.Platforms) == 0 {
		return errors.New("platforms are empty")
	}
	for _, platform := range r.Platforms {
		if platform == "tg" && len(r.Attachments) == 0 && len(r.Text) > 4096 {
			return errors.New("text is too long for telegram")
		}
		if platform == "tg" && len(r.Attachments) > 0 && len(r.Text) > 1024 {
			return errors.New("text is too long for telegram with attachments")
		}
		if platform == "vk" && len(r.Text) > 16384 {
			return errors.New("text is too long for vkontakte")
		}
		if platform == "fb" && len(r.Text) > 63206 {
			return errors.New("text is too long for facebook")
		}
		if platform == "ok" && len(r.Text) > 32000 {
			return errors.New("text is too long for odnoklassniki")
		}
		if platform == "ig" && len(r.Text) > 2200 {
			return errors.New("text is too long for instagram")
		}
	}
	return nil
}

type EditPostRequest struct {
	UserID      int
	TeamID      int
	PostUnionID int    `json:"post_union_id"`
	Text        string `json:"text"`
}

type DeletePostRequest struct {
	UserID      int
	TeamID      int
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
	UserID      int
	TeamID      int
	PostUnionID int    `db:"post_union_id"`
	Operation   string `db:"op"`
	Platform    string `db:"platform"`
}

type PostAction struct {
	ID          int       `db:"id"`
	PostUnionID int       `db:"post_union_id"`
	Operation   string    `db:"op"`
	Platform    string    `db:"platform"`
	Status      string    `db:"status"`
	ErrMessage  string    `db:"error_message"`
	CreatedAt   time.Time `db:"created_at"`
}

type PostUnionList struct {
	Posts []*PostUnion `json:"posts"`
}

type PostPlatform struct {
	ID          int    `db:"id"`
	PostUnionId int    `db:"post_union_id"`
	PostId      int    `db:"post_id"`
	Platform    string `db:"platform"`
}

type PostStatusRequest struct {
	UserID      int
	TeamID      int
	PostUnionID int `db:"post_union_id"`
}

type PostActionResponse struct {
	PostID     int    `json:"post_id"`
	Platform   string `json:"platform"`
	Status     string `json:"status"`
	ErrMessage string `json:"err_message"`
}
