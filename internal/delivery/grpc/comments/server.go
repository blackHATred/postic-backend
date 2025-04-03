package comments

import (
	"github.com/labstack/gommon/log"
	commentsProto "postic-backend/internal/delivery/grpc/comments/proto"
	"postic-backend/internal/usecase"
)

type Grpc struct {
	commentsProto.UnimplementedCommentsServer
	telegramEventListener usecase.Listener
}

func NewGrpc(telegramEventListener usecase.Listener) *Grpc {
	return &Grpc{telegramEventListener: telegramEventListener}
}

func (g *Grpc) Subscribe(req *commentsProto.SubscribeRequest, stream commentsProto.Comments_SubscribeServer) error {
	log.Infof("Subscribe request received: %v", req)
	// Пример отправки обновлений комментариев
	updates := g.telegramEventListener.SubscribeToCommentEvents(int(req.GetTeamId()), int(req.GetPostUnionId()))
	defer g.telegramEventListener.UnsubscribeFromComments(int(req.GetTeamId()), int(req.GetPostUnionId()))

	for update := range updates {
		commentUpdate := &commentsProto.AffectedComment{}
		commentUpdate.Id = int64(update)
		if err := stream.Send(commentUpdate); err != nil {
			return err
		}
	}

	return nil
}
