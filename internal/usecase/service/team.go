package service

import (
	"errors"
	"github.com/SevereCloud/vksdk/v3/api"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"postic-backend/internal/usecase"
	"slices"
	"time"
)

type Team struct {
	teamRepo repo.Team
}

func NewTeam(teamRepo repo.Team) usecase.Team {
	return &Team{
		teamRepo: teamRepo,
	}
}

func (t *Team) GetUserTeams(userID int) ([]*entity.Team, error) {
	teamIDs, err := t.teamRepo.GetUserTeams(userID)
	if err != nil {
		return nil, err
	}
	teams := make([]*entity.Team, 0, len(teamIDs))
	for _, teamID := range teamIDs {
		team, err := t.teamRepo.GetTeam(teamID)
		if errors.Is(err, repo.ErrTeamNotFound) {
			continue // команда не найдена, пропускаем
		}
		if err != nil {
			return nil, err
		}
		team.Users = make([]*entity.TeamUserRole, 0)
		// для каждой команды получаем пользователей и заполняем их роли
		users, err := t.teamRepo.GetTeamUsers(teamID)
		if err != nil {
			return nil, err
		}
		for _, userID_ := range users {
			roles, err := t.teamRepo.GetTeamUserRoles(teamID, userID_)
			if err != nil {
				return nil, err
			}
			team.Users = append(team.Users, &entity.TeamUserRole{
				TeamID: teamID,
				UserID: userID_,
				Roles:  roles,
			})
		}
		teams = append(teams, team)
	}
	return teams, nil
}

func (t *Team) GetTeamSecret(userID, teamID int) (string, error) {
	// сначала проверяем, что есть права админа
	roles, err := t.teamRepo.GetTeamUserRoles(teamID, userID)
	if err != nil {
		return "", err
	}
	if !slices.Contains(roles, repo.AdminRole) {
		return "", usecase.ErrUserForbidden
	}
	team, err := t.teamRepo.GetTeam(teamID)
	if errors.Is(err, repo.ErrTeamNotFound) {
		return "", usecase.ErrTeamNotFound
	}
	if err != nil {
		return "", err
	}
	return team.Secret, nil
}

func (t *Team) CreateTeam(request *entity.CreateTeamRequest) (int, error) {
	if len(request.TeamName) == 0 || len(request.TeamName) > 64 {
		return 0, usecase.ErrTeamNameLenIncorrect
	}
	teamID, err := t.teamRepo.AddTeam(&entity.Team{
		Name:      request.TeamName,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return 0, err
	}
	// добавляем админа в команду
	err = t.teamRepo.EditTeamUserRoles(teamID, request.RequestUserID, []string{repo.AdminRole})
	if errors.Is(err, repo.ErrUserNotFound) {
		return 0, usecase.ErrUserNotFound
	}
	if err != nil {
		return 0, err
	}
	return teamID, nil
}

func (t *Team) UpdateRoles(request *entity.UpdateRolesRequest) error {
	// проверяем, что админ команды
	roles, err := t.teamRepo.GetTeamUserRoles(request.TeamID, request.RequestUserID)
	if err != nil {
		return err
	}
	if !slices.Contains(roles, repo.AdminRole) {
		return usecase.ErrUserForbidden
	}
	// проверяем, что в запросе перечислены лишь доступные роли
	availableRoles := []string{repo.AdminRole, repo.PostsRole, repo.CommentsRole, repo.AnalyticsRole}
	for _, role := range request.Roles {
		if !slices.Contains(availableRoles, role) {
			return usecase.ErrRoleDoesNotExist
		}
	}
	// самому себе пользователь поменять роли не может
	if request.UserID == request.RequestUserID {
		return usecase.ErrUserForbidden
	}
	// обновляем роли
	err = t.teamRepo.EditTeamUserRoles(request.TeamID, request.UserID, request.Roles)
	if errors.Is(err, repo.ErrUserNotFound) {
		return usecase.ErrUserNotFound
	}
	if err != nil {
		return err
	}
	return nil
}

func (t *Team) RenameTeam(request *entity.RenameTeamRequest) error {
	// проверяем, что админ команды
	roles, err := t.teamRepo.GetTeamUserRoles(request.TeamID, request.RequestUserID)
	if err != nil {
		return err
	}
	if !slices.Contains(roles, repo.AdminRole) {
		return usecase.ErrUserForbidden
	}
	// проверяем, что имя команды не слишком длинное
	if len(request.NewName) == 0 || len(request.NewName) > 64 {
		return usecase.ErrTeamNameLenIncorrect
	}
	// получаем команду
	team, err := t.teamRepo.GetTeam(request.TeamID)
	if errors.Is(err, repo.ErrTeamNotFound) {
		return usecase.ErrTeamNotFound
	}
	if err != nil {
		return err
	}
	team.Name = request.NewName
	// обновляем имя команды
	err = t.teamRepo.EditTeam(team)
	if err != nil {
		return err
	}
	return nil
}

func (t *Team) InviteUser(request *entity.UpdateRolesRequest) error {
	// проверяем, что админ команды
	roles, err := t.teamRepo.GetTeamUserRoles(request.TeamID, request.RequestUserID)
	if err != nil {
		return err
	}
	if !slices.Contains(roles, repo.AdminRole) {
		return usecase.ErrUserForbidden
	}
	// проверяем, что в запросе перечислены лишь доступные роли
	availableRoles := []string{repo.AdminRole, repo.PostsRole, repo.CommentsRole, repo.AnalyticsRole}
	for _, role := range request.Roles {
		if !slices.Contains(availableRoles, role) {
			return usecase.ErrRoleDoesNotExist
		}
	}
	// обновляем роли
	err = t.teamRepo.EditTeamUserRoles(request.TeamID, request.UserID, request.Roles)
	if errors.Is(err, repo.ErrUserNotFound) {
		return usecase.ErrUserNotFound
	}
	if err != nil {
		return err
	}
	return nil
}

func (t *Team) Kick(request *entity.KickUserRequest) error {
	// проверяем, что админ команды
	roles, err := t.teamRepo.GetTeamUserRoles(request.TeamID, request.RequestUserID)
	if err != nil {
		return err
	}
	if !slices.Contains(roles, repo.AdminRole) && request.RequestUserID != request.KickedUserID {
		return usecase.ErrUserForbidden
	}
	// удаляем роли пользователя
	err = t.teamRepo.DeleteTeamUserRoles(request.TeamID, request.KickedUserID)
	if err != nil {
		return err
	}
	return nil
}

func (t *Team) Platforms(userID, teamID int) (*entity.TeamPlatforms, error) {
	// проверяем, что админ команды
	roles, err := t.teamRepo.GetTeamUserRoles(teamID, userID)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		// нет ролей - не может посмотреть привязанные платформы
		return nil, usecase.ErrUserForbidden
	}
	// получаем платформы команды
	platforms := &entity.TeamPlatforms{}
	// telegram
	tgChannel, err := t.teamRepo.GetTGChannelByTeamID(teamID)
	switch {
	case errors.Is(err, repo.ErrTGChannelNotFound):
		break
	case err != nil:
		return nil, err
	}
	platforms.TGChannelID = tgChannel.ChannelID
	platforms.TGDiscussionID = 0
	if tgChannel.DiscussionID != nil {
		platforms.TGDiscussionID = *tgChannel.DiscussionID
	}
	// vkontakte
	vkChannel, err := t.teamRepo.GetVKCredsByTeamID(teamID)
	switch {
	case errors.Is(err, repo.ErrVKChannelNotFound):
		break
	case err != nil:
		return nil, err
	}
	platforms.VKGroupID = vkChannel.GroupID
	return platforms, nil
}

func (t *Team) SetVK(request *entity.SetVKRequest) error {
	// Проверяем, что пользователь админ команды
	roles, err := t.teamRepo.GetTeamUserRoles(request.TeamID, request.RequestUserID)
	if err != nil {
		return err
	}
	if !slices.Contains(roles, repo.AdminRole) {
		return usecase.ErrUserForbidden
	}
	// Проверяем валидность пользовательского токена
	vkAdmin := api.NewVK(request.AdminApiKey)
	userInfo, err := vkAdmin.UsersGet(api.Params{})
	if err != nil {
		return errors.Join(usecase.ErrUserForbidden, errors.New("invalid admin token"))
	}
	if len(userInfo) == 0 {
		return errors.Join(usecase.ErrUserForbidden, errors.New("invalid admin token"))
	}

	// Проверяем, что пользователь является администратором указанной группы
	groupInfo, err := vkAdmin.GroupsGetByID(api.Params{
		"group_id": request.GroupID,
		"fields":   "is_admin",
	})
	if err != nil {
		return err
	}
	if len(groupInfo.Groups) == 0 || !groupInfo.Groups[0].IsAdmin {
		return errors.Join(usecase.ErrUserForbidden, errors.New("user is not an admin of the group"))
	}

	// Проверяем валидность токена сообщества
	vkGroup := api.NewVK(request.GroupApiKey)
	groupInfo, err = vkGroup.GroupsGetByID(api.Params{
		"group_id": request.GroupID,
	})
	if err != nil {
		return errors.Join(usecase.ErrUserForbidden, errors.New("invalid group token"))
	}
	if len(groupInfo.Groups) == 0 {
		return errors.Join(usecase.ErrUserForbidden, errors.New("invalid group token or group not found"))
	}

	// Сохраняем данные в репозиторий
	err = t.teamRepo.PutVKGroup(&entity.VKChannel{
		TeamID:      request.TeamID,
		GroupID:     request.GroupID,
		AdminAPIKey: request.AdminApiKey,
		GroupAPIKey: request.GroupApiKey,
	})
	if err != nil {
		return err
	}

	return nil
}
