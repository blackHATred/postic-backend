package vkontakte

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"strings"
	"sync"
	"time"

	"github.com/SevereCloud/vksdk/v3/api"
	"github.com/SevereCloud/vksdk/v3/events"
	longpoll "github.com/SevereCloud/vksdk/v3/longpoll-bot"
	"github.com/SevereCloud/vksdk/v3/object"
	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
	"github.com/labstack/gommon/log"
)

const vkVideoBaseURL = "https://vk.com/video%d_%d"

type EventListener struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	vkontakteListenerRepo repo.VkontakteListener
	teamRepo              repo.Team
	postRepo              repo.Post
	uploadUseCase         usecase.Upload
	commentRepo           repo.Comment
	mu                    sync.Mutex
	eventRepo             repo.CommentEventRepository // Kafka-репозиторий событий
	lpClients             map[int]*longpoll.LongPoll
	vkClients             map[int]*api.VK
	stopCh                chan struct{}
	ticker                *time.Ticker
}

func NewVKEventListener(
	vkontakteListenerRepo repo.VkontakteListener,
	teamRepo repo.Team,
	postRepo repo.Post,
	uploadUseCase usecase.Upload,
	commentRepo repo.Comment,
	eventRepo repo.CommentEventRepository,
) usecase.Listener {
	ctx, cancel := context.WithCancel(context.Background())
	return &EventListener{
		ctx:                   ctx,
		cancel:                cancel,
		vkontakteListenerRepo: vkontakteListenerRepo,
		teamRepo:              teamRepo,
		postRepo:              postRepo,
		uploadUseCase:         uploadUseCase,
		commentRepo:           commentRepo,
		eventRepo:             eventRepo,
		lpClients:             make(map[int]*longpoll.LongPoll),
		vkClients:             make(map[int]*api.VK),
		stopCh:                make(chan struct{}),
	}
}

func (e *EventListener) StartListener() {
	// Запускаем тикер для периодической проверки новых групп
	e.ticker = time.NewTicker(1 * time.Minute)
	go func() {
		// Сразу проверяем при старте
		e.checkForUnwatchedGroups()

		for {
			select {
			case <-e.ticker.C:
				// Подтверждаем наблюдение за группами
				go func() {
					for k := range e.lpClients {
						err := e.vkontakteListenerRepo.UpdateGroupLastUpdate(k)
						if err != nil {
							log.Errorf("Failed to update group last update timestamp: %e", err)
						}
					}
				}()
				// Проверяем группы, которые давно не обновлялись
				go e.checkForUnwatchedGroups()
			case <-e.stopCh:
				return
			}
		}
	}()
}

func (e *EventListener) StopListener() {
	if e.ticker != nil {
		e.ticker.Stop()
	}

	// Отменяем контекст
	e.cancel()

	// Останавливаем все лонгполлы
	e.mu.Lock()
	for _, lp := range e.lpClients {
		lp.Shutdown()
	}
	e.mu.Unlock()

	close(e.stopCh)
}

func (e *EventListener) checkForUnwatchedGroups() {
	// Получаем список команд, у которых давно не обновлялся статус VK
	teams, err := e.vkontakteListenerRepo.GetUnwatchedGroups(5 * time.Minute)
	if err != nil {
		log.Errorf("Failed to get unwatched VK groups: %e", err)
		return
	}

	for _, teamID := range teams {
		// Получаем данные для подключения к VK API
		vkChannel, err := e.teamRepo.GetVKCredsByTeamID(teamID)
		if err != nil {
			log.Errorf("Failed to get VK credentials for team %d: %e", teamID, err)
			continue
		}

		// Обновляем статус отслеживания группы
		err = e.vkontakteListenerRepo.UpdateGroupLastUpdate(teamID)
		if err != nil {
			log.Errorf("Failed to update group last update timestamp: %e", err)
			continue
		}
		// Настраиваем лонгполл для команды, если его еще нет
		e.mu.Lock()
		if _, exists := e.lpClients[teamID]; !exists {
			vk := api.NewVK(vkChannel.AdminAPIKey)
			lp, err := longpoll.NewLongPoll(vk, vkChannel.GroupID)
			if err != nil {
				log.Errorf("Failed to create longpoll for team %d: %e", teamID, err)
				e.mu.Unlock()
				continue
			}

			// Получаем последний сохранённый timestamp для этой команды
			lastEventTS, err := e.vkontakteListenerRepo.GetLastEventTS(teamID)
			if err != nil {
				log.Errorf("Failed to get last event timestamp for team %d: %v", teamID, err)
				lastEventTS = "0" // Используем начальное значение по умолчанию
			}

			// Устанавливаем timestamp для продолжения с последнего обработанного события
			lp.Ts = lastEventTS
			log.Infof("Starting VK longpoll for team %d from timestamp %s", teamID, lastEventTS)

			e.setupLongPollHandlers(lp, teamID)

			e.lpClients[teamID] = lp
			e.vkClients[teamID] = vk

			go func(teamID int, lp *longpoll.LongPoll) {
				err := lp.RunWithContext(e.ctx)
				if err != nil {
					log.Errorf("Longpoll for team %d stopped with error: %e", teamID, err)
					e.mu.Lock()
					delete(e.lpClients, teamID)
					delete(e.vkClients, teamID)
					e.mu.Unlock()
				}
			}(teamID, lp)
		}
		e.mu.Unlock()
	}
}

func (e *EventListener) setupLongPollHandlers(lp *longpoll.LongPoll, teamID int) {
	// Сохраняем timestamp после обработки каждого события
	lp.FullResponse(func(resp longpoll.Response) {
		if resp.Ts != "" {
			e.saveLastEventTS(teamID, resp.Ts)
		}
	})

	lp.WallReplyNew(func(ctx context.Context, object events.WallReplyNewObject) {
		e.wallReplyNewHandler(ctx, object, teamID)
	})
	lp.WallReplyDelete(func(ctx context.Context, object events.WallReplyDeleteObject) {
		e.wallReplyDeleteHandler(ctx, object, teamID)
	})
	lp.WallReplyEdit(func(ctx context.Context, object events.WallReplyEditObject) {
		e.wallReplyEditHandler(ctx, object, teamID)
	})
	lp.WallReplyRestore(func(ctx context.Context, object events.WallReplyRestoreObject) {
		e.wallReplyRestoreHandler(ctx, object, teamID)
	})
	/*
		DEPRECATED
		lp.LikeAdd(func(ctx context.Context, object events.LikeAddObject) {
			e.likeAddHandler(ctx, object, teamID)
		})
		lp.LikeRemove(func(ctx context.Context, object events.LikeRemoveObject) {
			e.likeRemoveHandler(ctx, object, teamID)
		})
	*/
}

func (e *EventListener) wallReplyNewHandler(ctx context.Context, obj events.WallReplyNewObject, teamID int) {
	vkChannel, err := e.teamRepo.GetVKCredsByTeamID(teamID)
	if err != nil {
		log.Errorf("Failed to get VK credentials: %v", err)
		return
	}
	postPlatform, err := e.postRepo.GetPostPlatformByPost(obj.PostID, vkChannel.ID, "vk")
	if errors.Is(err, repo.ErrPostPlatformNotFound) {
		return // Игнорируем комментарии к постам, которые мы не отслеживаем
	}
	if err != nil {
		log.Errorf("Failed to get post platform: %v", err)
		return
	}

	userInfo, err := e.getUserInfo(teamID, obj.FromID)
	if err != nil {
		log.Errorf("Failed to get user info: %v", err)
		return
	}

	newComment := &entity.Comment{
		TeamID:            teamID,
		PostUnionID:       &postPlatform.PostUnionId,
		Platform:          "vk",
		PostPlatformID:    &obj.PostID,
		UserPlatformID:    obj.FromID,
		CommentPlatformID: obj.ID,
		FullName:          userInfo.FullName,
		Username:          userInfo.Username,
		Text:              obj.Text,
		CreatedAt:         time.Unix(int64(obj.Date), 0),
	}

	// Возможно, это реплай на один из существующих комментариев
	if obj.ReplyToComment != 0 {
		replyComment, err := e.commentRepo.GetCommentByPlatformID(obj.ReplyToComment, "vk")
		if err != nil {
			log.Errorf("Failed to get comment: %v", err)
			return
		}
		newComment.ReplyToCommentID = replyComment.ID
	}

	newComment.AvatarMediaFile, err = e.getUserAvatar(userInfo.Avatar)
	if err != nil {
		log.Errorf("Failed to get user avatar: %v", err)
		// ошибка не фатальна, продолжаем
	}

	// Обрабатываем аттачи
	if len(obj.Attachments) > 0 {
		attachments, videosURL, err := e.processVKAttachments(obj.Attachments)
		if err != nil {
			log.Errorf("Failed to process attachments: %v", err)
		} else {
			if len(videosURL) > 0 {
				newComment.Text += "\n📎Пользователь прикрепил видео: " + strings.Join(videosURL, ", ")
			}
			if len(attachments) > 0 {
				uploads := make([]*entity.Upload, len(attachments))
				for i, attachment := range attachments {
					uploads[i] = &entity.Upload{
						ID: attachment,
					}
				}
				newComment.Attachments = uploads
			}
		}
	}

	// Сохраняем комментарий
	commentID, err := e.commentRepo.AddComment(newComment)
	if err != nil {
		log.Errorf("Failed to save comment: %v", err)
		return
	}

	// Уведомляем подписчиков о новом комментарии
	event := &entity.CommentEvent{
		EventID:    fmt.Sprintf("vk-%d-%d", teamID, commentID),
		TeamID:     teamID,
		PostID:     postPlatform.PostUnionId,
		Type:       entity.CommentCreated,
		CommentID:  commentID,
		OccurredAt: newComment.CreatedAt,
	}
	err = e.eventRepo.PublishCommentEvent(ctx, event)
	if err != nil {
		log.Errorf("Failed to notify subscribers: %v", err)
	}
}

func (e *EventListener) wallReplyDeleteHandler(ctx context.Context, obj events.WallReplyDeleteObject, teamID int) {
	// Находим комментарий в нашей БД
	comment, err := e.commentRepo.GetCommentByPlatformID(obj.ID, "vk")
	if errors.Is(err, repo.ErrCommentNotFound) {
		return // Комментарий не найден, так что ничего не делаем
	}
	if err != nil {
		log.Errorf("Failed to get comment: %v", err)
		return
	}

	// Удаляем комментарий
	err = e.commentRepo.DeleteComment(comment.ID)
	if err != nil {
		log.Errorf("Failed to delete comment: %v", err)
		return
	}

	// Уведомляем подписчиков об удаленном комментарии
	postUnionID := 0
	if comment.PostUnionID != nil {
		postUnionID = *comment.PostUnionID
	}
	event := &entity.CommentEvent{
		EventID:    fmt.Sprintf("vk-%d-%d", teamID, comment.ID),
		TeamID:     teamID,
		PostID:     postUnionID,
		Type:       entity.CommentDeleted,
		CommentID:  comment.ID,
		OccurredAt: time.Now(),
	}
	err = e.eventRepo.PublishCommentEvent(ctx, event)
	if err != nil {
		log.Errorf("Failed to notify subscribers: %v", err)
	}
}

func (e *EventListener) wallReplyEditHandler(ctx context.Context, obj events.WallReplyEditObject, teamID int) {
	comment, err := e.commentRepo.GetCommentByPlatformID(obj.ID, "vk")
	if errors.Is(err, repo.ErrCommentNotFound) {
		return // Комментарий не найден, ничего не делаем
	}
	if err != nil {
		log.Errorf("Failed to get comment: %v", err)
		return
	}

	// Обновляем текст комментария
	comment.Text = obj.Text

	// Обрабатываем аттачи, если таковые имеются
	if len(obj.Attachments) > 0 {
		attachments, videosURL, err := e.processVKAttachments(obj.Attachments)
		if err != nil {
			log.Errorf("Failed to process attachments: %v", err)
		} else {
			uploads := make([]*entity.Upload, len(attachments))
			for i, attachment := range attachments {
				uploads[i] = &entity.Upload{
					ID: attachment,
				}
			}
			comment.Attachments = uploads
		}
		if len(videosURL) > 0 {
			comment.Text += "\n📎Пользователь прикрепил видео: " + strings.Join(videosURL, ", ")
		}
	}

	// Сохраняем обновленный комментарий
	err = e.commentRepo.EditComment(comment)
	if err != nil {
		log.Errorf("Failed to update comment: %v", err)
		return
	}

	// Уведомляем подписчиков об обновленном комментарии
	postUnionID := 0
	if comment.PostUnionID != nil {
		postUnionID = *comment.PostUnionID
	}
	event := &entity.CommentEvent{
		EventID:    fmt.Sprintf("vk-%d-%d", teamID, comment.ID),
		TeamID:     teamID,
		PostID:     postUnionID,
		Type:       entity.CommentEdited,
		CommentID:  comment.ID,
		OccurredAt: time.Now(),
	}
	err = e.eventRepo.PublishCommentEvent(ctx, event)
	if err != nil {
		log.Errorf("Failed to notify subscribers: %v", err)
	}
}

func (e *EventListener) wallReplyRestoreHandler(ctx context.Context, obj events.WallReplyRestoreObject, teamID int) {
	// Это аналогично новому комментарию, но сначала проверяем, существует ли он уже
	existingComment, err := e.commentRepo.GetCommentByPlatformID(obj.ID, "vk")
	if err == nil {
		// Комментарий существует, просто помечаем его как активный
		// Для этого просто обновляем его текст
		err = e.commentRepo.EditComment(existingComment)
		if err != nil {
			log.Errorf("Failed to restore comment: %v", err)
		}
		return
	}

	// Если комментарий не существует, то создаем новый
	vkChannel, err := e.teamRepo.GetVKCredsByTeamID(teamID)
	if err != nil {
		log.Errorf("Failed to get VK credentials: %v", err)
		return
	}
	postPlatform, err := e.postRepo.GetPostPlatformByPost(obj.PostID, vkChannel.ID, "vk")
	if errors.Is(err, repo.ErrPostPlatformNotFound) {
		return // Игнорим комментарии, которые мы не отслеживаем
	}
	if err != nil {
		log.Errorf("Failed to get post platform: %v", err)
		return
	}

	// Получаем инфу о пользователе
	userInfo, err := e.getUserInfo(teamID, obj.FromID)
	if err != nil {
		log.Errorf("Failed to get user info: %v", err)
		return
	}

	// Создаем новый комментарий
	newComment := &entity.Comment{
		TeamID:            teamID,
		PostUnionID:       &postPlatform.PostUnionId,
		Platform:          "vk",
		PostPlatformID:    &obj.PostID,
		UserPlatformID:    obj.FromID,
		CommentPlatformID: obj.ID,
		FullName:          userInfo.FullName,
		Username:          userInfo.Username,
		Text:              obj.Text,
		CreatedAt:         time.Unix(int64(obj.Date), 0),
	}

	newComment.AvatarMediaFile, err = e.getUserAvatar(userInfo.Avatar)
	if err != nil {
		log.Errorf("Failed to get user avatar: %v", err)
		// ошибка не фатальна, продолжаем
	}

	// Сохраняем комментарий
	commentID, err := e.commentRepo.AddComment(newComment)
	if err != nil {
		log.Errorf("Failed to save restored comment: %v", err)
		return
	}

	// Уведомляем подписчиков о новом комментарии (даже несмотря на то, что он восстановленный)
	event := &entity.CommentEvent{
		EventID:    fmt.Sprintf("vk-%d-%d", teamID, commentID),
		TeamID:     teamID,
		PostID:     postPlatform.PostUnionId,
		Type:       entity.CommentCreated,
		CommentID:  commentID,
		OccurredAt: newComment.CreatedAt,
	}
	err = e.eventRepo.PublishCommentEvent(ctx, event)
	if err != nil {
		log.Errorf("Failed to notify subscribers: %v", err)
	}
}

type UserInfo struct {
	FullName string
	Username string
	Avatar   string
}

func (e *EventListener) getUserInfo(teamID, userID int) (*UserInfo, error) {
	vk, ok := e.vkClients[teamID]
	if !ok {
		return nil, fmt.Errorf("VK client not found for team ID %d", teamID)
	}
	user, err := vk.UsersGet(api.Params{
		"user_ids": userID,
		"fields":   "photo_200,domain,first_name,last_name",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	if len(user) == 0 {
		return nil, errors.New("user not found")
	}
	userName := user[0].Domain
	fullName := fmt.Sprintf("%s %s", user[0].FirstName, user[0].LastName)
	avatar := user[0].Photo200
	return &UserInfo{
		FullName: fullName,
		Username: userName,
		Avatar:   avatar,
	}, nil
}

// processVKAttachments возвращает ID загруженных файлов, а для видео - url на их воспроизведение
func (e *EventListener) processVKAttachments(attachments []object.WallCommentAttachment) ([]int, []string, error) {
	var url string
	var fileType string
	uploads := make([]int, 0, len(attachments))
	videosURL := make([]string, 0, len(attachments))

	for _, attachment := range attachments {
		switch attachment.Type {
		case "photo":
			if len(attachment.Photo.Sizes) == 0 {
				continue
			}
			url = attachment.Photo.Sizes[len(attachment.Photo.Sizes)-1].URL
			fileType = "photo"
		case "video":
			if attachment.Video.Player == "" {
				// Собираем ссылку вручную
				url = fmt.Sprintf(vkVideoBaseURL, attachment.Video.OwnerID, attachment.Video.ID)
			} else {
				url = attachment.Video.Player
			}
			fileType = "video"
			videosURL = append(videosURL, url)
			log.Infof("Video URL: %s", url)
			continue
		case "sticker":
			if attachment.Sticker.AnimationURL == "" && len(attachment.Sticker.Images) == 0 {
				continue
			}
			url = attachment.Sticker.AnimationURL
			if url == "" {
				url = attachment.Sticker.Images[len(attachment.Sticker.Images)-1].URL
			}
			fileType = "sticker"
		case "doc":
			url = attachment.Doc.URL
			fileType = "doc"
		default:
			return nil, nil, fmt.Errorf("unsupported attachment type: %s", attachment.Type)
		}

		// Получаем содержимое файла
		resp, err := http.Get(url)
		if err != nil {
			log.Errorf("Failed to get file content: %v", err)
			return nil, nil, err
		}
		// Читаем всё содержимое в память
		data, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			log.Errorf("Failed to read file content: %v", err)
			return nil, nil, err
		}
		// Определяем MIME-тип
		mime := mimetype.Detect(data)
		if err != nil {
			log.Errorf("Failed to detect MIME type: %v", err)
			return nil, nil, err
		}
		extension := strings.TrimPrefix(mime.Extension(), ".")
		// Сохраняем в S3
		upload := &entity.Upload{
			RawBytes: bytes.NewReader(data),
			FilePath: fmt.Sprintf("vk/%s.%s", uuid.New().String(), extension),
			FileType: fileType,
		}
		uploadFileId, err := e.uploadUseCase.UploadFile(upload)
		if err != nil {
			log.Errorf("Failed to upload file: %v", err)
			return nil, nil, err
		}
		uploads = append(uploads, uploadFileId)
	}
	return uploads, videosURL, nil
}

func (e *EventListener) getUserAvatar(avatarUrl string) (*entity.Upload, error) {
	resp, err := http.Get(avatarUrl)
	if err != nil {
		log.Errorf("Failed to get file content: %v", err)
		return nil, err
	}

	// Читаем содержимое в буфер
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Failed to read response body: %v", err)
		return nil, err
	}
	_ = resp.Body.Close()

	extension := "jpg"
	upload := &entity.Upload{
		RawBytes: bytes.NewReader(body),
		FilePath: fmt.Sprintf("vk/%s.%s", uuid.New().String(), extension),
		FileType: "photo",
	}
	uploadFileId, err := e.uploadUseCase.UploadFile(upload)
	if err != nil {
		log.Errorf("Failed to upload file: %v", err)
		return nil, err
	}
	upload.ID = uploadFileId
	return upload, nil
}

// saveLastEventTS сохраняет timestamp последнего обработанного события для команды
func (e *EventListener) saveLastEventTS(teamID int, ts string) {
	err := e.vkontakteListenerRepo.SetLastEventTS(teamID, ts)
	if err != nil {
		log.Errorf("Failed to save last event timestamp %s for team %d: %v", ts, teamID, err)
	}
}
