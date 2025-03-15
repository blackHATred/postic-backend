package main

import (
	"context"
	"github.com/SevereCloud/vksdk/v3/api"
	"github.com/SevereCloud/vksdk/v3/events"
	"github.com/SevereCloud/vksdk/v3/longpoll-bot"
	"log"
)

func main() {
	vk := api.NewVK("token")
	lp, err := longpoll.NewLongPoll(vk, 0)
	if err != nil {
		panic(err)
	}

	log.Println("LongPoll created")
	lp.WallReplyNew(func(ctx context.Context, obj events.WallReplyNewObject) {
		log.Print(obj.Text)
	})
	lp.MessageNew(func(ctx context.Context, obj events.MessageNewObject) {
		log.Print(obj.Message.Text)
	})

	lp.Run()
}
