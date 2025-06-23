package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"slices"
	"strings"
	"time"

	"github.com/labstack/gommon/log"
)

type PostUnion struct {
	postRepo        repo.Post
	teamRepo        repo.Team
	uploadUseCase   usecase.Upload
	analyticsRepo   repo.Analytics
	telegram        usecase.PostPlatform
	vkontakte       usecase.PostPlatform
	generatePostURL string
	fixPostTextURL  string
}

func NewPostUnion(
	postRepo repo.Post,
	teamRepo repo.Team,
	uploadUseCase usecase.Upload,
	analyticsRepo repo.Analytics,
	telegram usecase.PostPlatform,
	vkontakte usecase.PostPlatform,
	generatePostURL string,
	fixPostTextURL string,
) usecase.PostUnion {
	p := &PostUnion{
		postRepo:        postRepo,
		teamRepo:        teamRepo,
		uploadUseCase:   uploadUseCase,
		analyticsRepo:   analyticsRepo,
		telegram:        telegram,
		vkontakte:       vkontakte,
		generatePostURL: generatePostURL,
		fixPostTextURL:  fixPostTextURL,
	}
	// запускаем горутину для мониторинга запланированных постов
	go p.scheduleListen()
	return p
}

func (p *PostUnion) scheduleListen() {
	// мониторим запланированные посты раз в 10 секунд
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		// получаем все запланированные посты, которые ждут публикации
		scheduledPosts, err := p.postRepo.GetScheduledPosts("pending", time.Now(), true, 5)
		if err != nil {
			log.Errorf("error getting scheduled posts: %v", err)
			continue
		}
		for _, scheduledPost := range scheduledPosts {
			log.Infof("scheduled posts: %v", *scheduledPost)
			if time.Now().After(scheduledPost.ScheduledAt) {
				// получаем необходимый пост
				postUnion, err := p.postRepo.GetPostUnion(scheduledPost.PostUnionID)
				log.Infof("scheduled post: %v", *postUnion)
				if err != nil {
					log.Errorf("error getting post union: %v", err)
				}
				// публикуем пост
				for _, platform := range postUnion.Platforms {
					// Создаем задачу на обновление статистики для каждой платформы
					err = p.analyticsRepo.CreateStatsUpdateTask(postUnion.ID, platform)
					if err != nil {
						log.Errorf("Ошибка создания задачи обновления статистики для %s: %v", platform, err)
					}
					switch platform {
					case "tg":
						// добавляем пост в телеграм
						_, err = p.telegram.AddPost(postUnion)
						if err != nil {
							log.Errorf("error adding post to telegram: %v", err)
							continue
						}
					case "vk":
						// добавляем пост во вконтакте
						_, err = p.vkontakte.AddPost(postUnion)
						if err != nil {
							log.Errorf("error adding post to vk: %v", err)
							continue
						}
						// другие платформы todo
					}
				}
				// обновляем запись о запланированном посте
				scheduledPost.Status = "published"
				err = p.postRepo.EditScheduledPost(scheduledPost)
				if err != nil {
					log.Errorf("error updating scheduled post: %v", err)
					continue
				}
			}
		}
	}
}

func (p *PostUnion) AddPostUnion(request *entity.AddPostRequest) (int, []int, error) {
	if err := request.IsValid(); err != nil {
		return 0, nil, err
	}
	// Проверяем, что пользователь админ или имеет отдельное право на публикации
	permissions, err := p.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return 0, nil, err
	}
	if !slices.Contains(permissions, repo.AdminRole) && !slices.Contains(permissions, repo.PostsRole) {
		return 0, nil, errors.New("user has no permission to create post")
	}

	// Создание записи в таблице post_union
	if request.PubDateTime != nil && request.PubDateTime.After(time.Now().Add(time.Hour*24*365)) {
		return 0, nil, errors.New("publication date is too far in the future")
	}
	attachments := make([]*entity.Upload, len(request.Attachments))
	if len(request.Attachments) > 0 {
		for i, attachment := range request.Attachments {
			upload, err := p.uploadUseCase.GetUpload(attachment)
			if err != nil {
				return 0, nil, err
			}
			attachments[i] = upload
		}
	}
	postUnion := &entity.PostUnion{
		UserID:      request.UserID,
		TeamID:      request.TeamID,
		Text:        request.Text,
		Platforms:   request.Platforms,
		CreatedAt:   time.Now(),
		PubDate:     request.PubDateTime,
		Attachments: attachments,
	}
	postUnionID, err := p.postRepo.AddPostUnion(postUnion)
	if err != nil {
		return 0, nil, err
	}
	postUnion.ID = postUnionID

	// Если pubdatetime > now, то создаем запланированную публикацию
	if request.PubDateTime != nil && request.PubDateTime.After(time.Now()) {
		_, err = p.postRepo.AddScheduledPost(&entity.ScheduledPost{
			PostUnionID: postUnionID,
			ScheduledAt: *request.PubDateTime,
			Status:      "pending",
			CreatedAt:   time.Now(),
		})
		// Так как никаких действий с внешними платформами пока не произошло, то возвращаем пустой список actions
		return postUnionID, []int{}, err
	}
	// Если pubdatetime <= now, то на каждой из платформ создаем action
	var actionIDs []int
	for _, platform := range request.Platforms {
		// Создаем задачу на обновление статистики для каждой платформы
		err = p.analyticsRepo.CreateStatsUpdateTask(postUnionID, platform)
		if err != nil {
			log.Errorf("Ошибка создания задачи обновления статистики для %s: %v", platform, err)
		}

		switch platform {
		case "tg":
			platformID, err := p.telegram.AddPost(postUnion)
			if err != nil {
				return postUnionID, actionIDs, err
			}
			actionIDs = append(actionIDs, platformID)
		case "vk":
			platformID, err := p.vkontakte.AddPost(postUnion)
			if err != nil {
				return postUnionID, actionIDs, err
			}
			actionIDs = append(actionIDs, platformID)
			// todo другие платформы
		}
	}
	return postUnionID, actionIDs, nil
}

func (p *PostUnion) EditPostUnion(request *entity.EditPostRequest) ([]int, error) {
	// редактировать можно только текст, неопубликованные посты, а также посты, с момента публикации которых
	// прошло не более суток

	// проверяем права пользователя
	permissions, err := p.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(permissions, repo.AdminRole) && !slices.Contains(permissions, repo.PostsRole) {
		return nil, usecase.ErrUserForbidden
	}
	// проверяем, что пост принадлежит этой команде
	postUnion, err := p.postRepo.GetPostUnion(request.PostUnionID)
	if err != nil {
		return nil, err
	}
	if postUnion.TeamID != request.TeamID {
		return nil, usecase.ErrUserForbidden
	}
	if (postUnion.PubDate != nil && time.Now().After(postUnion.PubDate.Add(time.Hour*24))) ||
		(postUnion.PubDate == nil && time.Now().After(postUnion.CreatedAt.Add(time.Hour*24))) {
		return nil, usecase.ErrPostUnavailableToEdit
	}
	if len(postUnion.Attachments) == 0 && strings.TrimSpace(request.Text) == "" {
		return nil, usecase.ErrPostTextAndAttachmentsAreRequired
	}

	// валидация длины текста для каждой платформы
	if err := request.IsValid(postUnion.Platforms); err != nil {
		return nil, err
	}
	// если это запланированный и пока что неопубликованный пост, то просто редактируем его
	if postUnion.PubDate != nil && postUnion.PubDate.After(time.Now()) {
		postUnion.Text = request.Text
		err = p.postRepo.EditPostUnion(postUnion)
		if err != nil {
			return nil, err
		}
		// новых action не произошло, поэтому возвращаем пустой слайс
		return []int{}, nil
	}
	// если это уже опубликованный пост, то создаем новый action на редактирование на всех платформах
	var actionIDs []int
	for _, platform := range postUnion.Platforms {
		switch platform {
		case "tg":
			actionID, err := p.telegram.EditPost(&entity.EditPostRequest{
				PostUnionID: request.PostUnionID,
				Text:        request.Text,
			})
			if err != nil {
				return nil, err
			}
			actionIDs = append(actionIDs, actionID)
		case "vk":
			actionID, err := p.vkontakte.EditPost(&entity.EditPostRequest{
				PostUnionID: request.PostUnionID,
				Text:        request.Text,
			})
			if err != nil {
				return nil, err
			}
			actionIDs = append(actionIDs, actionID)
			// todo другие платформы
		}
	}
	// обновляем текст поста в базе данных
	postUnion.Text = request.Text
	err = p.postRepo.EditPostUnion(postUnion)
	if err != nil {
		return nil, err
	}
	return actionIDs, nil
}

func (p *PostUnion) DeletePostUnion(request *entity.DeletePostRequest) ([]int, error) {
	// проверяем права пользователя
	permissions, err := p.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(permissions, repo.AdminRole) && !slices.Contains(permissions, repo.PostsRole) {
		return nil, usecase.ErrUserForbidden
	}
	// проверяем, что пост принадлежит этой команде
	postUnion, err := p.postRepo.GetPostUnion(request.PostUnionID)
	if err != nil {
		return nil, err
	}
	if postUnion.TeamID != request.TeamID {
		return nil, usecase.ErrUserForbidden
	}
	var actionIDs []int
	for _, platform := range postUnion.Platforms {
		switch platform {
		case "tg":
			actionID, err := p.telegram.DeletePost(&entity.DeletePostRequest{
				UserID:      request.UserID,
				TeamID:      request.TeamID,
				PostUnionID: request.PostUnionID,
			})
			if err != nil {
				return nil, err
			}
			actionIDs = append(actionIDs, actionID)
		case "vk":
			actionID, err := p.vkontakte.DeletePost(&entity.DeletePostRequest{
				UserID:      request.UserID,
				TeamID:      request.TeamID,
				PostUnionID: request.PostUnionID,
			})
			if err != nil {
				return nil, err
			}
			actionIDs = append(actionIDs, actionID)
			// todo другие платформы
		}
	}
	return actionIDs, nil
}

func (p *PostUnion) GetPostUnion(request *entity.GetPostRequest) (*entity.PostUnion, error) {
	// проверяем права пользователя
	permissions, err := p.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(permissions, repo.AdminRole) && !slices.Contains(permissions, repo.PostsRole) {
		return nil, usecase.ErrUserForbidden
	}
	// проверяем, что пост принадлежит этой команде
	postUnion, err := p.postRepo.GetPostUnion(request.PostUnionID)
	if err != nil {
		return nil, err
	}
	if postUnion.TeamID != request.TeamID {
		return nil, usecase.ErrUserForbidden
	}
	return postUnion, nil
}

func (p *PostUnion) GetPosts(request *entity.GetPostsRequest) ([]*entity.PostUnion, error) {
	// проверяем права пользователя
	permissions, err := p.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(permissions, repo.AdminRole) && !slices.Contains(permissions, repo.PostsRole) {
		return nil, usecase.ErrUserForbidden
	}
	if request.Limit > 100 {
		request.Limit = 100
	}
	// получаем посты
	offset := time.Now()
	if request.Offset != nil {
		offset = *request.Offset
	}
	posts, err := p.postRepo.GetPostUnions(request.TeamID, offset, request.Before, request.Limit, request.Filter)
	if err != nil {
		return nil, err
	}
	return posts, nil
}

func (p *PostUnion) GetPostStatus(request *entity.PostStatusRequest) ([]*entity.PostActionResponse, error) {
	// проверяем права пользователя
	permissions, err := p.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(permissions, repo.AdminRole) && !slices.Contains(permissions, repo.PostsRole) {
		return nil, usecase.ErrUserForbidden
	}
	// проверяем, что пост принадлежит этой команде
	postUnion, err := p.postRepo.GetPostUnion(request.PostUnionID)
	if err != nil {
		return nil, err
	}
	if postUnion.TeamID != request.TeamID {
		return nil, usecase.ErrUserForbidden
	}

	actionIDs, err := p.postRepo.GetPostActions(request.PostUnionID)
	if err != nil {
		return nil, err
	}
	responses := make([]*entity.PostActionResponse, len(actionIDs))

	for i, actionID := range actionIDs {
		action, err := p.postRepo.GetPostAction(actionID)
		if err != nil {
			return nil, err
		}
		responses[i] = &entity.PostActionResponse{
			PostID:     request.PostUnionID,
			Platform:   action.Platform,
			Operation:  action.Operation,
			Status:     action.Status,
			ErrMessage: action.ErrMessage,
			CreatedAt:  action.CreatedAt,
		}
	}

	return responses, nil
}

func (p *PostUnion) DoAction(request *entity.DoActionRequest) (int, error) {
	// проверяем права пользователя
	permissions, err := p.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return 0, err
	}
	if !slices.Contains(permissions, repo.AdminRole) && !slices.Contains(permissions, repo.PostsRole) {
		return 0, usecase.ErrUserForbidden
	}
	// проверяем, что пост принадлежит этой команде
	postUnion, err := p.postRepo.GetPostUnion(request.PostUnionID)
	if err != nil {
		return 0, err
	}
	if postUnion.TeamID != request.TeamID {
		return 0, usecase.ErrUserForbidden
	}

	switch request.Operation {
	case "add":
		switch request.Platform {
		case "tg":
			actionID, err := p.telegram.AddPost(postUnion)
			if err != nil {
				return 0, err
			}
			return actionID, nil
		case "vk":
			actionID, err := p.vkontakte.AddPost(postUnion)
			if err != nil {
				return 0, err
			}
			return actionID, nil
		}
	case "delete":
		switch request.Platform {
		case "tg":
			actionID, err := p.telegram.DeletePost(&entity.DeletePostRequest{
				UserID:      request.UserID,
				TeamID:      request.TeamID,
				PostUnionID: request.PostUnionID,
			})
			if err != nil {
				return 0, err
			}
			return actionID, nil
		case "vk":
			actionID, err := p.vkontakte.DeletePost(&entity.DeletePostRequest{
				UserID:      request.UserID,
				TeamID:      request.TeamID,
				PostUnionID: request.PostUnionID,
			})
			if err != nil {
				return 0, err
			}
			return actionID, nil
		}
	case "edit":
		switch request.Platform {
		case "tg":
			actionID, err := p.telegram.EditPost(&entity.EditPostRequest{
				UserID:      request.UserID,
				TeamID:      request.TeamID,
				PostUnionID: request.PostUnionID,
				Text:        postUnion.Text,
			})
			if err != nil {
				return 0, err
			}
			return actionID, nil
		case "vk":
			actionID, err := p.vkontakte.EditPost(&entity.EditPostRequest{
				UserID:      request.UserID,
				TeamID:      request.TeamID,
				PostUnionID: request.PostUnionID,
				Text:        postUnion.Text,
			})
			if err != nil {
				return 0, err
			}
			return actionID, nil
		}
	}

	return 0, nil
}

// GeneratePost генерирует пост с помощью AI через SSE
func (p *PostUnion) GeneratePost(request *entity.GeneratePostRequest) (<-chan string, error) {
	// проверяем права пользователя
	roles, err := p.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(roles, repo.AdminRole) && !slices.Contains(roles, repo.PostsRole) {
		return nil, usecase.ErrUserForbidden
	}

	// создаем канал для SSE
	sseChannel := make(chan string, 100)

	// запускаем горутину для выполнения запроса к postic-ml
	go func() {
		defer close(sseChannel)

		type MLRequest struct {
			Query string `json:"query"`
		}

		jsonData, err := json.Marshal(MLRequest{Query: request.Query})
		if err != nil {
			sseChannel <- fmt.Sprintf("data: {\"error\": \"Ошибка сериализации запроса: %s\"}\n\n", err.Error())
			return
		}

		req, err := http.NewRequest("POST", p.generatePostURL, bytes.NewBuffer(jsonData))
		if err != nil {
			sseChannel <- fmt.Sprintf("data: {\"error\": \"Ошибка создания запроса: %s\"}\n\n", err.Error())
			return
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			sseChannel <- fmt.Sprintf("data: {\"error\": \"Ошибка выполнения запроса: %s\"}\n\n", err.Error())
			return
		}
		defer resp.Body.Close()

		// читаем SSE поток
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line != "" {
				sseChannel <- line + "\n"
			}
		}

		if err := scanner.Err(); err != nil {
			sseChannel <- fmt.Sprintf("data: {\"error\": \"Ошибка чтения потока: %s\"}\n\n", err.Error())
		}
	}()

	return sseChannel, nil
}

// FixPostText исправляет ошибки в тексте поста
func (p *PostUnion) FixPostText(request *entity.FixPostTextRequest) (*entity.FixPostTextResponse, error) {
	// проверяем права пользователя
	roles, err := p.teamRepo.GetTeamUserRoles(request.TeamID, request.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(roles, repo.AdminRole) && !slices.Contains(roles, repo.PostsRole) {
		return nil, usecase.ErrUserForbidden
	}

	type MLRequest struct {
		Text string `json:"text"`
	}

	jsonData, err := json.Marshal(MLRequest{Text: request.Text})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", p.fixPostTextURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	type MLResponse struct {
		Response string `json:"response"`
	}

	var mlResponse MLResponse
	err = json.NewDecoder(resp.Body).Decode(&mlResponse)
	if err != nil {
		return nil, err
	}

	return &entity.FixPostTextResponse{
		Text: mlResponse.Response,
	}, nil
}
