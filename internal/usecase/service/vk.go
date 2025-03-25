package service

import (
	"github.com/SevereCloud/vksdk/v3/api"
	"github.com/labstack/gommon/log"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"time"
)

type VK struct {
	postRepo    repo.Post
	userRepo    repo.User
	uploadRepo  repo.Upload
	postActions chan entity.PostAction
}

func (v *VK) AddPostInQueue(postAction entity.PostAction) error {
	v.postActions <- postAction
	return nil
}

func (v *VK) postActionQueue() {
	for action := range v.postActions {
		go v.post(action)
	}
}

func (v *VK) post(action entity.PostAction) {
	// TODO статусы потом
	/*	postActionID, err := v.postRepo.AddPostAction(&action)
		if err != nil {
			log.Errorf("TG POST: AddPostAction failed: %v", err)
			return
		}*/

	postUnion, err := v.postRepo.GetPostUnion(action.PostUnionID)
	if err != nil {
		log.Errorf("VK POST: GetPostUnion failed: %v", err)
		return
	}

	vkChannel, err := v.userRepo.GetVKChannel(postUnion.UserID)
	if err != nil {
		log.Errorf("VK POST: GetVKChannel failed: %v", err)
		return
	}

	vk := api.NewVK(vkChannel.APIKey)

	params := api.Params{
		"owner_id": -vkChannel.GroupID, // ID группы с -
		"message":  postUnion.Text,
		"guid":     action.ID,
	}
	log.Printf("VK POST: PostUnion: %v", postUnion)

	if postUnion.PubDate.Unix() > 0 && postUnion.PubDate.After(time.Now()) {
		params["publish_date"] = postUnion.PubDate.Unix()
	}

	response, err := vk.WallPost(params)
	if err != nil {
		log.Errorf("VK POST: WallPost failed: %v", err)
		_ = v.postRepo.EditPostActionStatus(action.ID, "error", err.Error())
		return
	}
	postID := response.PostID

	err = v.postRepo.EditPostActionStatus(action.ID, "success", "")
	if err != nil {
		log.Errorf("VK POST: EditPostActionStatus failed: %v", err)
	}
	err = v.postRepo.AddPostVK(postUnion.ID, postID)
}

func NewVK(postRepo repo.Post, userRepo repo.User, uploadRepo repo.Upload) (usecase.Platform, error) {
	vkUC := &VK{
		postRepo:    postRepo,
		userRepo:    userRepo,
		uploadRepo:  uploadRepo,
		postActions: make(chan entity.PostAction),
	}
	go vkUC.postActionQueue()
	return vkUC, nil
}
