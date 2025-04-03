package usecase

type Listener interface {
	StartListener()
	StopListener()
	// SubscribeToCommentEvents подписывается на комментарии к посту и возвращает канал, по которому будут
	//приходить ID новых, измененных или удаленных комментариев
	SubscribeToCommentEvents(teamId, postUnionId int) <-chan int
	UnsubscribeFromComments(teamId, postUnionId int)
}
