package repo

type Team interface {
	// GetTeamIDBySecret возвращает ID команды по секретному ключу
	GetTeamIDBySecret(secret string) (int, error)
	// GetTGChannelByTeamID возвращает ID телеграм канала по ID команды
	GetTGChannelByTeamID(teamId int) (int, error)
	// PutTGChannel привязывает ID телеграм канала и ID обсуждения к команде
	PutTGChannel(teamId int, channelId int, discussionId int) error
	// GetTGChannelByDiscussionId возвращает ID телеграм канала (в нашей системе, а не реальный) по ID обсуждения
	GetTGChannelByDiscussionId(discussionId int) (int, error)
	// GetTeamIDByPostUnionID возвращает ID команды, которая может видеть пост с данным ID
	GetTeamIDByPostUnionID(postUnionID int) (int, error)
	// GetUserPermissionsByTeamID возвращает права пользователя в команде
	GetUserPermissionsByTeamID(teamId int, userId int) ([]UserTeamRole, error)
}

type UserTeamRole string

const (
	AdminRole     = "admin"
	PostsRole     = "posts"
	CommentsRole  = "comments"
	AnalyticsRole = "analytics"
)
