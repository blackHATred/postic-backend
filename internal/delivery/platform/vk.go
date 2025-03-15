package platform

import (
	"context"
	"fmt"
	"github.com/SevereCloud/vksdk/v3/api"
	"github.com/SevereCloud/vksdk/v3/events"
	"github.com/SevereCloud/vksdk/v3/longpoll-bot"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"sync"
	"time"
)

type VkClientPair struct {
	api *api.VK
	lp  *longpoll.LongPoll
}

type Vk struct {
	groupIDs    map[int]VkClientPair
	chats       map[int]chan entity.Message
	mu          sync.Mutex
	commentRepo repo.Comment
}

func NewVk() (*Vk, error) {
	return &Vk{chats: make(map[int]chan entity.Message), groupIDs: make(map[int]VkClientPair)}, nil
}

func (v *Vk) AddGroup(token string, groupId int) (<-chan entity.Message, error) {
	vk := api.NewVK(token)
	lp, err := longpoll.NewLongPoll(vk, groupId)
	if err != nil {
		return nil, err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.groupIDs[groupId] = VkClientPair{api: vk, lp: lp}
	v.chats[groupId] = make(chan entity.Message)
	fmt.Printf("Vk group %d added\n", groupId)
	go func() {
		err := v.ListenGroup(groupId)
		if err != nil {
			fmt.Println(err)
		}
	}()
	return v.chats[groupId], nil
}

func (v *Vk) ListenGroup(groupId int) error {
	v.mu.Lock()
	lp := v.groupIDs[groupId].lp
	v.mu.Unlock()
	lp.WallReplyNew(func(ctx context.Context, obj events.WallReplyNewObject) {
		// получаем имя пользователя и его аватарку
		username, avatar, err := v.getUserNameAndAvatar(groupId, obj.FromID)
		if err != nil {
			fmt.Println(err)
			return
		}
		v.chats[groupId] <- entity.Message{Id: obj.ID, Text: obj.Text, Type: "new", Username: username, Time: time.Now().Format(time.RFC3339), Platform: "vk", AvatarURL: avatar}
	})
	lp.WallReplyDelete(func(ctx context.Context, obj events.WallReplyDeleteObject) {
		v.chats[groupId] <- entity.Message{Id: obj.ID, Text: "", Type: "delete", Time: time.Now().Format(time.RFC3339), Platform: "vk"}
	})
	lp.WallReplyEdit(func(ctx context.Context, obj events.WallReplyEditObject) {
		v.chats[groupId] <- entity.Message{Id: obj.ID, Text: obj.Text, Type: "update", Time: time.Now().Format(time.RFC3339), Platform: "vk"}
	})
	fmt.Printf("Vk group %d started\n", groupId)
	return lp.Run()
}

func (v *Vk) getUserNameAndAvatar(groupId, userId int) (string, string, error) {
	v.mu.Lock()
	vk := v.groupIDs[groupId].api
	v.mu.Unlock()
	user, err := vk.UsersGet(api.Params{"user_ids": userId})
	if err != nil {
		return "", "", err
	}
	return user[0].FirstName + " " + user[0].LastName, user[0].Photo200, nil
}

func (v *Vk) AddGroupMock(token string, groupId int) (<-chan entity.Message, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.groupIDs[groupId] = VkClientPair{}
	v.chats[groupId] = make(chan entity.Message)
	fmt.Printf("Vk group %d added\n", groupId)
	return v.chats[groupId], nil
}

func (v *Vk) RemoveGroup(chatID int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.chats, chatID)
}

func (v *Vk) ListenMock() error {
	// с помощью ticker время от времени отправляем сообщения в канал
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		for _, ch := range v.chats {
			ch <- entity.Message{Id: 1, Text: "Hello!", Type: "new", Username: "Username", Time: "2021-10-01T12:00:00Z", Platform: "vk"}
		}
	}
	return nil
}
