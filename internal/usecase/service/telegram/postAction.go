package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/labstack/gommon/log"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"postic-backend/pkg/retry"
	"time"
)

type Telegram struct {
	bot        *tgbotapi.BotAPI
	postRepo   repo.Post
	teamRepo   repo.Team
	uploadRepo repo.Upload
}

func NewTelegram(
	token string,
	postRepo repo.Post,
	teamRepo repo.Team,
	uploadRepo repo.Upload,
) (usecase.PostPlatform, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &Telegram{
		bot:        bot,
		postRepo:   postRepo,
		teamRepo:   teamRepo,
		uploadRepo: uploadRepo,
	}, nil
}

func (t *Telegram) createPostAction(request *entity.PostUnion) (int, error) {
	var postActionId int
	err := retry.Retry(func() error {
		var err error
		postActionId, err = t.postRepo.AddPostAction(&entity.PostAction{
			PostUnionID: request.ID,
			Operation:   "publish",
			Platform:    "tg",
			Status:      "pending",
			CreatedAt:   time.Now(),
		})
		return err
	})
	return postActionId, err
}

func (t *Telegram) updatePostActionStatus(actionId int, request *entity.PostUnion, status, errMsg string) {
	// Иногда могут возникать ошибки, но они не должны прерывать выполнение ввиду асинхронности бизнес-логики.
	// Поэтому экспоненциально делаем ретраи и логируем ошибки
	err := retry.Retry(func() error {
		return t.postRepo.EditPostAction(&entity.PostAction{
			ID:          actionId,
			PostUnionID: request.ID,
			Operation:   "publish",
			Platform:    "tg",
			Status:      status,
			ErrMessage:  errMsg,
			CreatedAt:   request.CreatedAt,
		})
	})
	if err != nil {
		log.Errorf("error while updating post action status: %v", err)
	}
}

func (t *Telegram) publishPost(request *entity.PostUnion, actionId int) {
	var tgChannelId int
	var err error
	// получаем id канала
	err = retry.Retry(func() error {
		tgChannelId, _, err = t.teamRepo.GetTGChannelByTeamID(request.TeamID)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.updatePostActionStatus(actionId, request, "error", err.Error())
		return
	}
	if tgChannelId == 0 {
		t.updatePostActionStatus(actionId, request, "error", "channel not found")
		return
	}

	if len(request.Attachments) == 0 {
		t.handleNoAttachments(request, actionId, tgChannelId)
	} else if len(request.Attachments) == 1 {
		t.handleSingleAttachment(request, actionId, tgChannelId)
	} else if len(request.Attachments) > 1 && len(request.Attachments) < 11 {
		t.handleMultipleAttachments(request, actionId, tgChannelId)
	} else {
		t.updatePostActionStatus(actionId, request, "error", "too many attachments")
		return
	}
}

func (t *Telegram) handleNoAttachments(request *entity.PostUnion, actionId, tgChannelId int) {
	if request.Text == "" {
		t.updatePostActionStatus(actionId, request, "error", "empty post")
		return
	}

	msg := tgbotapi.NewMessage(int64(tgChannelId), request.Text)
	_, err := t.bot.Send(msg)
	if err != nil {
		t.updatePostActionStatus(actionId, request, "error", err.Error())
		return
	}

	t.updatePostActionStatus(actionId, request, "success", "")
}

func (t *Telegram) handleSingleAttachment(request *entity.PostUnion, actionId, tgChannelId int) {
	attachment := request.Attachments[0]
	upload, err := t.uploadRepo.GetUpload(attachment.ID)
	if err != nil {
		t.updatePostActionStatus(actionId, request, "error", err.Error())
		return
	}

	switch attachment.FileType {
	case "photo":
		t.sendPhoto(request, actionId, tgChannelId, upload)
	case "video":
		t.sendVideo(request, actionId, tgChannelId, upload)
	}
}

func (t *Telegram) handleMultipleAttachments(request *entity.PostUnion, actionId, tgChannelId int) {
	var mediaGroup []any
	for i, attachment := range request.Attachments {
		upload, err := t.uploadRepo.GetUpload(attachment.ID)
		if err != nil {
			t.updatePostActionStatus(actionId, request, "error", err.Error())
			return
		}

		var media any
		switch attachment.FileType {
		case "photo":
			photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FileReader{
				Name:   upload.FilePath,
				Reader: upload.RawBytes,
			})
			if i == 0 {
				photo.Caption = request.Text
			}
			media = photo
		case "video":
			video := tgbotapi.NewInputMediaVideo(tgbotapi.FileReader{
				Name:   upload.FilePath,
				Reader: upload.RawBytes,
			})
			if i == 0 {
				video.Caption = request.Text
			}
			media = video
		}
		mediaGroup = append(mediaGroup, media)
	}

	if len(mediaGroup) > 0 {
		msg := tgbotapi.NewMediaGroup(int64(tgChannelId), mediaGroup)
		err := retry.Retry(func() error {
			_, err := t.bot.Send(msg)
			return err
		})
		if err != nil {
			t.updatePostActionStatus(actionId, request, "error", err.Error())
			return
		}
	}

	t.updatePostActionStatus(actionId, request, "success", "")
}

func (t *Telegram) sendPhoto(request *entity.PostUnion, actionId, tgChannelId int, upload *entity.Upload) {
	req := tgbotapi.NewPhoto(int64(tgChannelId), tgbotapi.FileReader{
		Name:   upload.FilePath,
		Reader: upload.RawBytes,
	})
	req.Caption = request.Text
	msg, err := t.bot.Send(req)
	if err != nil {
		t.updatePostActionStatus(actionId, request, "error", err.Error())
		return
	}

	err = retry.Retry(func() error {
		_, err := t.postRepo.AddPostPlatform(&entity.PostPlatform{
			PostUnionId: request.ID,
			PostId:      msg.MessageID,
			Platform:    "tg",
		})
		return err
	})
	if err != nil {
		log.Errorf("error while adding post platform: %v", err)
	}

	t.updatePostActionStatus(actionId, request, "success", "")
}

func (t *Telegram) sendVideo(request *entity.PostUnion, actionId, tgChannelId int, upload *entity.Upload) {
	req := tgbotapi.NewVideo(int64(tgChannelId), tgbotapi.FileReader{
		Name:   upload.FilePath,
		Reader: upload.RawBytes,
	})
	req.Caption = request.Text
	msg, err := t.bot.Send(req)
	if err != nil {
		log.Errorf("error while adding post video: %v", err)
		t.updatePostActionStatus(actionId, request, "error", err.Error())
		return
	}

	err = retry.Retry(func() error {
		_, err := t.postRepo.AddPostPlatform(&entity.PostPlatform{
			PostUnionId: request.ID,
			PostId:      msg.MessageID,
			Platform:    "tg",
		})
		return err
	})
	if err != nil {
		log.Errorf("error while adding post platform: %v", err)
	}

	t.updatePostActionStatus(actionId, request, "success", "")
}

func (t *Telegram) AddPost(request *entity.PostUnion) (int, error) {
	actionId, err := t.createPostAction(request)
	if err != nil {
		return 0, err
	}

	go t.publishPost(request, actionId)

	return actionId, nil
}

func (t *Telegram) EditPost(request *entity.EditPostRequest) (int, error) {
	var postActionId int
	err := retry.Retry(func() error {
		var err error
		postActionId, err = t.postRepo.AddPostAction(&entity.PostAction{
			PostUnionID: request.PostUnionID,
			Operation:   "edit",
			Platform:    "tg",
			Status:      "pending",
			CreatedAt:   time.Now(),
		})
		return err
	})
	if err != nil {
		return 0, err
	}

	post, err := t.postRepo.GetPostUnion(request.PostUnionID)
	if err != nil {
		t.updatePostActionStatus(postActionId, post, "error", err.Error())
		return 0, err
	}

	var tgChannelId int
	err = retry.Retry(func() error {
		var err error
		tgChannelId, _, err = t.teamRepo.GetTGChannelByTeamID(post.TeamID)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.updatePostActionStatus(postActionId, post, "error", err.Error())
		return 0, err
	}

	postPlatform, err := t.postRepo.GetPostPlatform(request.PostUnionID, "tg")
	if err != nil {
		t.updatePostActionStatus(postActionId, post, "error", err.Error())
		return 0, err
	}

	// Start asynchronous edit operation
	go t.editPostAsync(post, postActionId, tgChannelId, postPlatform.PostId, request.Text)

	return postActionId, nil
}

func (t *Telegram) editPostAsync(post *entity.PostUnion, actionId, tgChannelId, messageId int, newText string) {
	// Если нет вложений, то просто обновляем текст
	if len(post.Attachments) == 0 {
		msg := tgbotapi.NewEditMessageText(int64(tgChannelId), messageId, newText)
		_, err := t.bot.Send(msg)
		if err != nil {
			t.updatePostActionStatus(actionId, post, "error", err.Error())
			return
		}
	} else {
		// Для постов с аттачами редактируем описание первого аттача
		editMsg := tgbotapi.NewEditMessageCaption(int64(tgChannelId), messageId, newText)
		_, err := t.bot.Send(editMsg)
		if err != nil {
			t.updatePostActionStatus(actionId, post, "error", err.Error())
			return
		}
	}

	t.updatePostActionStatus(actionId, post, "success", "")
}

func (t *Telegram) DeletePost(request *entity.DeletePostRequest) (int, error) {
	var postActionId int
	err := retry.Retry(func() error {
		var err error
		postActionId, err = t.postRepo.AddPostAction(&entity.PostAction{
			PostUnionID: request.PostUnionID,
			Operation:   "delete",
			Platform:    "tg",
			Status:      "pending",
			CreatedAt:   time.Now(),
		})
		return err
	})
	if err != nil {
		return 0, err
	}

	post, err := t.postRepo.GetPostUnion(request.PostUnionID)
	if err != nil {
		t.updatePostActionStatus(postActionId, post, "error", err.Error())
		return 0, err
	}

	var tgChannelId int
	err = retry.Retry(func() error {
		var err error
		tgChannelId, _, err = t.teamRepo.GetTGChannelByTeamID(post.TeamID)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.updatePostActionStatus(postActionId, post, "error", err.Error())
		return 0, err
	}

	go t.deletePostAsync(post, postActionId, tgChannelId)

	return postActionId, nil
}

func (t *Telegram) deletePostAsync(post *entity.PostUnion, actionId, tgChannelId int) {
	// Получаем ID поста в телеграме
	postPlatform, err := t.postRepo.GetPostPlatform(post.ID, "tg")
	if err != nil {
		t.updatePostActionStatus(actionId, post, "error", err.Error())
		return
	}

	msg := tgbotapi.NewDeleteMessage(int64(tgChannelId), postPlatform.PostId)
	_, err = t.bot.Send(msg)
	if err != nil {
		t.updatePostActionStatus(actionId, post, "error", err.Error())
		return
	}

	t.updatePostActionStatus(actionId, post, "success", "")
}

func (t *Telegram) DoAction(request *entity.DoActionRequest) ([]int, error) {
	//TODO implement me
	panic("implement me")
}
