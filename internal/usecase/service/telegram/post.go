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

type Post struct {
	bot        *tgbotapi.BotAPI
	postRepo   repo.Post
	teamRepo   repo.Team
	uploadRepo repo.Upload
}

func NewTelegramPost(
	bot *tgbotapi.BotAPI,
	postRepo repo.Post,
	teamRepo repo.Team,
	uploadRepo repo.Upload,
) usecase.PostPlatform {
	return &Post{
		bot:        bot,
		postRepo:   postRepo,
		teamRepo:   teamRepo,
		uploadRepo: uploadRepo,
	}
}

func (p *Post) createPostAction(request *entity.PostUnion) (int, error) {
	var postActionId int
	err := retry.Retry(func() error {
		var err error
		postActionId, err = p.postRepo.AddPostAction(&entity.PostAction{
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

func (p *Post) updatePostActionStatus(actionId int, status, errMsg string) {
	// Иногда могут возникать ошибки, но они не должны прерывать выполнение ввиду асинхронности бизнес-логики.
	// Поэтому экспоненциально делаем ретраи и логируем ошибки
	err := retry.Retry(func() error {
		action, err := p.postRepo.GetPostAction(actionId)
		if err != nil {
			log.Errorf("error getting post action: %v", err)
			return err
		}
		return p.postRepo.EditPostAction(&entity.PostAction{
			ID:          actionId,
			PostUnionID: action.PostUnionID,
			Operation:   action.Operation,
			Platform:    action.Platform,
			Status:      status,
			ErrMessage:  errMsg,
			CreatedAt:   action.CreatedAt,
		})
	})
	if err != nil {
		log.Errorf("error while updating post action status: %v", err)
	}
}

func (p *Post) publishPost(request *entity.PostUnion, actionId int) {
	var tgChannelId int
	var err error
	// получаем id канала
	err = retry.Retry(func() error {
		tgChannelId, _, err = p.teamRepo.GetTGChannelByTeamID(request.TeamID)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		p.updatePostActionStatus(actionId, "error", err.Error())
		return
	}
	if tgChannelId == 0 {
		p.updatePostActionStatus(actionId, "error", "channel not found")
		return
	}

	if len(request.Attachments) == 0 {
		p.handleNoAttachments(request, actionId, tgChannelId)
	} else if len(request.Attachments) == 1 {
		p.handleSingleAttachment(request, actionId, tgChannelId)
	} else if len(request.Attachments) > 1 && len(request.Attachments) < 11 {
		p.handleMultipleAttachments(request, actionId, tgChannelId)
	} else {
		p.updatePostActionStatus(actionId, "error", "too many attachments")
		return
	}
}

func (p *Post) handleNoAttachments(request *entity.PostUnion, actionId, tgChannelId int) {
	if request.Text == "" {
		p.updatePostActionStatus(actionId, "error", "empty post")
		return
	}

	newMsg := tgbotapi.NewMessage(int64(tgChannelId), request.Text)
	msg, err := p.bot.Send(newMsg)
	if err != nil {
		p.updatePostActionStatus(actionId, "error", err.Error())
		return
	}

	err = retry.Retry(func() error {
		_, err := p.postRepo.AddPostPlatform(&entity.PostPlatform{
			PostUnionId: request.ID,
			PostId:      msg.MessageID,
			Platform:    "tg",
		})
		return err
	})
	if err != nil {
		log.Errorf("error while adding post platform: %v", err)
	}

	p.updatePostActionStatus(actionId, "success", "")
}

func (p *Post) handleSingleAttachment(request *entity.PostUnion, actionId, tgChannelId int) {
	attachment := request.Attachments[0]
	upload, err := p.uploadRepo.GetUpload(attachment.ID)
	if err != nil {
		p.updatePostActionStatus(actionId, "error", err.Error())
		return
	}

	switch attachment.FileType {
	case "photo":
		p.sendPhoto(request, actionId, tgChannelId, upload)
	case "video":
		p.sendVideo(request, actionId, tgChannelId, upload)
	}
}

func (p *Post) handleMultipleAttachments(request *entity.PostUnion, actionId, tgChannelId int) {
	var mediaGroup []any
	for i, attachment := range request.Attachments {
		upload, err := p.uploadRepo.GetUpload(attachment.ID)
		if err != nil {
			p.updatePostActionStatus(actionId, "error", err.Error())
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
		mediaGroupMsg := tgbotapi.NewMediaGroup(int64(tgChannelId), mediaGroup)
		var messages []tgbotapi.Message
		err := retry.Retry(func() error {
			var err error
			messages, err = p.bot.SendMediaGroup(mediaGroupMsg)
			return err
		})
		if err != nil {
			p.updatePostActionStatus(actionId, "error", err.Error())
			return
		}

		if len(messages) > 0 {
			tgMediaGroupMessages := make([]entity.TgPostPlatformGroup, len(messages)-1)
			for i, msg := range messages[1:] {
				tgMediaGroupMessages[i] = entity.TgPostPlatformGroup{
					PostPlatformID: messages[0].MessageID,
					TgPostID:       msg.MessageID,
				}
			}
			err := retry.Retry(func() error {
				_, err := p.postRepo.AddPostPlatform(&entity.PostPlatform{
					PostUnionId:         request.ID,
					PostId:              messages[0].MessageID,
					Platform:            "tg",
					TgPostPlatformGroup: tgMediaGroupMessages,
				})
				return err
			})
			if err != nil {
				log.Errorf("error while adding post platform: %v", err)
			}
		}
	}

	p.updatePostActionStatus(actionId, "success", "")
}

func (p *Post) sendPhoto(request *entity.PostUnion, actionId, tgChannelId int, upload *entity.Upload) {
	req := tgbotapi.NewPhoto(int64(tgChannelId), tgbotapi.FileReader{
		Name:   upload.FilePath,
		Reader: upload.RawBytes,
	})
	req.Caption = request.Text
	msg, err := p.bot.Send(req)
	if err != nil {
		p.updatePostActionStatus(actionId, "error", err.Error())
		return
	}

	err = retry.Retry(func() error {
		_, err := p.postRepo.AddPostPlatform(&entity.PostPlatform{
			PostUnionId: request.ID,
			PostId:      msg.MessageID,
			Platform:    "tg",
		})
		return err
	})
	if err != nil {
		log.Errorf("error while adding post platform: %v", err)
	}

	p.updatePostActionStatus(actionId, "success", "")
}

func (p *Post) sendVideo(request *entity.PostUnion, actionId, tgChannelId int, upload *entity.Upload) {
	req := tgbotapi.NewVideo(int64(tgChannelId), tgbotapi.FileReader{
		Name:   upload.FilePath,
		Reader: upload.RawBytes,
	})
	req.Caption = request.Text
	msg, err := p.bot.Send(req)
	if err != nil {
		log.Errorf("error while adding post video: %v", err)
		p.updatePostActionStatus(actionId, "error", err.Error())
		return
	}

	err = retry.Retry(func() error {
		_, err := p.postRepo.AddPostPlatform(&entity.PostPlatform{
			PostUnionId: request.ID,
			PostId:      msg.MessageID,
			Platform:    "tg",
		})
		return err
	})
	if err != nil {
		log.Errorf("error while adding post platform: %v", err)
	}

	p.updatePostActionStatus(actionId, "success", "")
}

func (p *Post) AddPost(request *entity.PostUnion) (int, error) {
	actionId, err := p.createPostAction(request)
	if err != nil {
		return 0, err
	}

	go p.publishPost(request, actionId)

	return actionId, nil
}

func (p *Post) EditPost(request *entity.EditPostRequest) (int, error) {
	var postActionId int
	err := retry.Retry(func() error {
		var err error
		postActionId, err = p.postRepo.AddPostAction(&entity.PostAction{
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

	post, err := p.postRepo.GetPostUnion(request.PostUnionID)
	if err != nil {
		p.updatePostActionStatus(postActionId, "error", err.Error())
		return 0, err
	}

	var tgChannelId int
	err = retry.Retry(func() error {
		var err error
		tgChannelId, _, err = p.teamRepo.GetTGChannelByTeamID(post.TeamID)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		p.updatePostActionStatus(postActionId, "error", err.Error())
		return 0, err
	}

	postPlatform, err := p.postRepo.GetPostPlatform(request.PostUnionID, "tg")
	if err != nil {
		p.updatePostActionStatus(postActionId, "error", err.Error())
		return 0, err
	}

	// Start asynchronous edit operation
	go p.editPostAsync(post, postActionId, tgChannelId, postPlatform.PostId, request.Text)

	return postActionId, nil
}

func (p *Post) editPostAsync(post *entity.PostUnion, actionId, tgChannelId, messageId int, newText string) {
	// Если нет вложений, то просто обновляем текст
	if len(post.Attachments) == 0 {
		msg := tgbotapi.NewEditMessageText(int64(tgChannelId), messageId, newText)
		_, err := p.bot.Send(msg)
		if err != nil {
			p.updatePostActionStatus(actionId, "error", err.Error())
			return
		}
	} else {
		// Для постов с аттачами редактируем описание первого аттача
		editMsg := tgbotapi.NewEditMessageCaption(int64(tgChannelId), messageId, newText)
		_, err := p.bot.Send(editMsg)
		if err != nil {
			p.updatePostActionStatus(actionId, "error", err.Error())
			return
		}
	}

	p.updatePostActionStatus(actionId, "success", "")
}

func (p *Post) DeletePost(request *entity.DeletePostRequest) (int, error) {
	var postActionId int
	err := retry.Retry(func() error {
		var err error
		postActionId, err = p.postRepo.AddPostAction(&entity.PostAction{
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

	post, err := p.postRepo.GetPostUnion(request.PostUnionID)
	if err != nil {
		p.updatePostActionStatus(postActionId, "error", err.Error())
		return 0, err
	}

	var tgChannelId int
	err = retry.Retry(func() error {
		var err error
		tgChannelId, _, err = p.teamRepo.GetTGChannelByTeamID(post.TeamID)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		p.updatePostActionStatus(postActionId, "error", err.Error())
		return 0, err
	}

	go p.deletePostAsync(post, postActionId, tgChannelId)

	return postActionId, nil
}

func (p *Post) deletePostAsync(post *entity.PostUnion, actionId, tgChannelId int) {
	// Получаем ID поста в телеграме
	postPlatform, err := p.postRepo.GetPostPlatform(post.ID, "tg")
	if err != nil {
		p.updatePostActionStatus(actionId, "error", err.Error())
		return
	}

	if postPlatform.TgPostPlatformGroup != nil && len(postPlatform.TgPostPlatformGroup) > 0 {
		// сначала удаляем все связанные в медиагруппе сообщения
		for _, tgPost := range postPlatform.TgPostPlatformGroup {
			msg := tgbotapi.NewDeleteMessage(int64(tgPost.TgPostID), tgPost.PostPlatformID)
			_, err = p.bot.Send(msg)
			if err != nil {
				p.updatePostActionStatus(actionId, "error", err.Error())
				return
			}
		}
	}
	msg := tgbotapi.NewDeleteMessage(int64(tgChannelId), postPlatform.PostId)
	_, err = p.bot.Send(msg)
	if err != nil {
		p.updatePostActionStatus(actionId, "error", err.Error())
		return
	}

	err = retry.Retry(func() error {
		return p.postRepo.DeletePlatformFromPostUnion(post.ID, "tg")
	})
	if err != nil {
		log.Errorf("error while deleting post platform: %v", err)
	}

	p.updatePostActionStatus(actionId, "success", "")
}
