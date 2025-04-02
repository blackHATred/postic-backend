package usecase

type TelegramListener interface {
	StartListener()
	StopListener()
	// SubscribeToCommentEvents подписывается на комментарии к посту в телеграме и возвращает канал, по которому будут
	//приходить ID новых, измененных или удаленных комментариев
	SubscribeToCommentEvents(teamId, postUnionId int) <-chan int
	UnsubscribeFromComments(teamId, postUnionId int)
}
