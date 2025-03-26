package repo

import "postic-backend/internal/entity"

type Channel interface {
	// GetTGChannelByDiscussionId возвращает канал Телеграм как канал публикации
	GetTGChannelByDiscussionId(discussionId int) (*entity.TGChannel, error)
}
