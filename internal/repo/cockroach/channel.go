package cockroach

import (
	"github.com/jmoiron/sqlx"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
)

type Channel struct {
	db *sqlx.DB
}

func NewChannel(db *sqlx.DB) repo.Channel {
	return &Channel{
		db: db,
	}
}

func (c Channel) GetTGChannelByDiscussionId(discussionId int) (*entity.TGChannel, error) {
	var channel entity.TGChannel
	query := `SELECT id, user_id, channel_id, discussion_id FROM channel_tg WHERE discussion_id = $1`
	err := c.db.Get(&channel, query, discussionId)
	if err != nil {
		return nil, err
	}
	return &channel, nil
}
