package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/labstack/gommon/log"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
)

type Telegram struct {
	bot        *tgbotapi.BotAPI
	postRepo   repo.Post
	teamRepo   repo.Team
	uploadRepo repo.Upload
}

func NewTelegram() usecase.PostPlatform {
	return &Telegram{}
}

func (t *Telegram) AddPost(request *entity.PostUnion) (int, error) {
	// создаем action в БД
	actionId, err := t.postRepo.AddPostAction(&entity.PostAction{
		PostUnionID: request.ID,
		Operation:   "publish",
		Platform:    "tg",
		Status:      "pending",
	})
	if err != nil {
		return 0, err
	}
	// создаем пост в телеграм в фоновом режиме
	go func() {
		// Получаем данные команды
		tgChannelId, err := t.teamRepo.GetTGChannelByTeamID(request.TeamID)
		if err != nil {
			log.Errorf("error while getting tg channel id: %v", err)
			// Обновляем статус action на error
			err := t.postRepo.EditPostAction(&entity.PostAction{
				ID:          actionId,
				PostUnionID: request.ID,
				Operation:   "publish",
				Platform:    "tg",
				Status:      "error",
				ErrMessage:  err.Error(),
			})
			if err != nil {
				log.Errorf("error while updating action status: %v", err)
			}
			return
		}
		// Прикрепляем файлы, если они есть
		// Если вложение одно
		if len(request.Attachments) == 1 {
			attachment := request.Attachments[0]
			// Получаем файл из БД
			upload, err := t.uploadRepo.GetUpload(attachment.ID)
			if err != nil {
				log.Errorf("error while getting upload: %v", err)
				// Обновляем статус action на error
				err := t.postRepo.EditPostAction(&entity.PostAction{
					ID:          actionId,
					PostUnionID: request.ID,
					Operation:   "publish",
					Platform:    "tg",
					Status:      "error",
					ErrMessage:  err.Error(),
				})
				if err != nil {
					log.Errorf("error while updating action status: %v", err)
				}
				return
			}
			if attachment.FileType == "photo" {
				req := tgbotapi.NewPhoto(int64(tgChannelId), tgbotapi.FileReader{
					Name:   upload.FilePath,
					Reader: upload.RawBytes,
				})
				req.Caption = request.Text
				msg, err := t.bot.Send(req)
				if err != nil {
					log.Errorf("error while sending photo: %v", err)
					// Обновляем статус action на error
					err := t.postRepo.EditPostAction(&entity.PostAction{
						ID:          actionId,
						PostUnionID: request.ID,
						Operation:   "publish",
						Platform:    "tg",
						Status:      "error",
						ErrMessage:  err.Error(),
					})
					if err != nil {
						log.Errorf("error while updating action status: %v", err)
					}
					return
				}
				_, err = t.postRepo.AddPostPlatform(&entity.PostPlatform{
					PostUnionId: request.ID,
					PostId:      msg.MessageID,
					Platform:    "tg",
				})
				if err != nil {
					log.Errorf("error while adding post platform: %v", err)
				}
				// Обновляем статус action на success
				err = t.postRepo.EditPostAction(&entity.PostAction{
					ID:          actionId,
					PostUnionID: request.ID,
					Operation:   "publish",
					Platform:    "tg",
					Status:      "success",
					ErrMessage:  "",
				})
				if err != nil {
					log.Errorf("error while updating action status: %v", err)
				}
			} else if attachment.FileType == "video" {
				req := tgbotapi.NewVideo(int64(tgChannelId), tgbotapi.FileReader{
					Name:   upload.FilePath,
					Reader: upload.RawBytes,
				})
				req.Caption = request.Text
				// todo
			}
		} else if len(request.Attachments) > 1 {
			// todo
		}

	}()
	return actionId, nil
}

func (t *Telegram) EditPost(request *entity.EditPostRequest) (int, error) {
	//TODO implement me
	panic("implement me")
}

func (t *Telegram) DeletePost(request *entity.DeletePostRequest) (int, error) {
	//TODO implement me
	panic("implement me")
}

func (t *Telegram) GetPostStatus(request *entity.PostStatusRequest) (*entity.PostActionResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (t *Telegram) DoAction(request *entity.DoActionRequest) ([]int, error) {
	//TODO implement me
	panic("implement me")
}
