package repo

type Team interface {
	// GetTeamIDBySecret возвращает ID команды по секретному ключу
	GetTeamIDBySecret(secret string) (int, error)
	// PutTGChannel привязывает ID телеграм канала и ID обсуждения к команде
	PutTGChannel(teamId int, channelId int, discussionId int) error
	// GetTGChannelByDiscussionId возвращает ID телеграм канала (в нашей системе, а не реальный) по ID обсуждения
	GetTGChannelByDiscussionId(discussionId int) (int, error)
	// GetTeamIDByPostUnionID возвращает ID команды, которая может видеть пост с данным ID
	GetTeamIDByPostUnionID(postUnionID int) (int, error)
}
