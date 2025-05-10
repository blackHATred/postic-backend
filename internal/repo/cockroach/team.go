package cockroach

import (
	"database/sql"
	"errors"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
)

type Team struct {
	db *sqlx.DB
}

func NewTeam(db *sqlx.DB) repo.Team {
	return &Team{db: db}
}

func (t *Team) GetTeamIDByVKGroupID(groupId int) (int, error) {
	var teamId int
	err := t.db.Get(&teamId, "SELECT team_id FROM channel_vk WHERE group_id = $1", groupId)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, repo.ErrTeamNotFound
	case err != nil:
		return 0, err
	}
	return teamId, nil
}

func (t *Team) GetTeamIDByTGDiscussionID(discussionId int) (int, error) {
	var teamId int
	err := t.db.Get(&teamId, "SELECT team_id FROM channel_tg WHERE discussion_id = $1", discussionId)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, repo.ErrTeamNotFound
	case err != nil:
		return 0, err
	}
	return teamId, nil
}

func (t *Team) GetTeamUsers(teamId int) ([]int, error) {
	var userIDs []int
	err := t.db.Select(&userIDs, "SELECT user_id FROM team_user_role WHERE team_id = $1", teamId)
	if err != nil {
		return nil, err
	}
	return userIDs, nil
}

func (t *Team) AddTeam(team *entity.Team) (int, error) {
	var id int
	err := t.db.QueryRow(
		"INSERT INTO team (name) VALUES ($1) RETURNING id",
		team.Name,
	).Scan(&id)
	return id, err
}

func (t *Team) EditTeam(team *entity.Team) error {
	_, err := t.db.Exec("UPDATE team SET name = $1 WHERE id = $2", team.Name, team.ID)
	return err
}

func (t *Team) GetTeam(teamId int) (*entity.Team, error) {
	team := &entity.Team{}
	err := t.db.Get(team, "SELECT id, name, secret, created_at FROM team WHERE id = $1", teamId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repo.ErrTeamNotFound
		}
		return nil, err
	}
	return team, nil
}

func (t *Team) GetUserTeams(userID int) ([]int, error) {
	var teamIDs []int
	err := t.db.Select(&teamIDs, "SELECT team_id FROM team_user_role WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	return teamIDs, nil
}

func (t *Team) GetTeamUserRoles(teamId int, userId int) ([]string, error) {
	var roles []string
	query := "SELECT roles FROM team_user_role WHERE team_id = $1 AND user_id = $2"
	if err := t.db.QueryRow(query, teamId, userId).Scan(pq.Array(&roles)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []string{}, nil
		}
		return nil, err
	}
	return roles, nil
}

func (t *Team) EditTeamUserRoles(teamId int, userId int, roles []string) error {
	_, err := t.db.Exec(
		"INSERT INTO team_user_role (team_id, user_id, roles) VALUES ($1, $2, $3) "+
			"ON CONFLICT (team_id, user_id) DO UPDATE SET roles = $3",
		teamId, userId, pq.Array(roles),
	)
	var pgErr *pq.Error
	if errors.As(err, &pgErr) && pgErr.Code.Name() == "foreign_key_violation" {
		return repo.ErrUserNotFound
	}
	return err
}

func (t *Team) DeleteTeamUserRoles(teamId int, userId int) error {
	_, err := t.db.Exec("DELETE FROM team_user_role WHERE team_id = $1 AND user_id = $2", teamId, userId)
	return err
}

func (t *Team) GetTeamIDBySecret(secret string) (int, error) {
	var teamID int
	err := t.db.Get(&teamID, "SELECT id FROM team WHERE secret = $1", secret)
	if err != nil {
		return 0, err
	}
	return teamID, nil
}

func (t *Team) GetTGChannelByTeamID(teamId int) (*entity.TGChannel, error) {
	var tgChannel entity.TGChannel
	err := t.db.QueryRow(
		"SELECT id, team_id, channel_id, discussion_id FROM channel_tg WHERE team_id = $1",
		teamId,
	).Scan(&tgChannel.ID, &tgChannel.TeamID, &tgChannel.ChannelID, &tgChannel.DiscussionID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repo.ErrTGChannelNotFound
		}
		return nil, err
	}

	return &tgChannel, nil
}

func (t *Team) PutTGChannel(tgChannel *entity.TGChannel) error {
	_, err := t.db.Exec(
		"INSERT INTO channel_tg (team_id, channel_id, discussion_id) VALUES ($1, $2, $3) "+
			"ON CONFLICT (team_id) DO UPDATE SET channel_id = $2, discussion_id = $3",
		tgChannel.TeamID, tgChannel.ChannelID, tgChannel.DiscussionID,
	)
	return err
}

func (t *Team) GetTGChannelByDiscussionId(discussionId int) (*entity.TGChannel, error) {
	var tgChannel entity.TGChannel
	err := t.db.QueryRow(
		"SELECT id, team_id, channel_id, discussion_id FROM channel_tg WHERE discussion_id = $1",
		discussionId,
	).Scan(&tgChannel.ID, &tgChannel.TeamID, &tgChannel.ChannelID, &tgChannel.DiscussionID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repo.ErrTGChannelNotFound
		}
		return nil, err
	}

	return &tgChannel, nil
}

func (t *Team) GetTeamIDByPostUnionID(postUnionID int) (int, error) {
	var teamId int
	err := t.db.Get(&teamId, "SELECT team_id FROM post_union WHERE id = $1", postUnionID)
	if err != nil {
		return 0, err
	}
	return teamId, nil
}

func (t *Team) PutVKGroup(vkChannel *entity.VKChannel) error {
	_, err := t.db.Exec(
		`INSERT INTO channel_vk (team_id, group_id, admin_api_key, group_api_key, last_updated_timestamp) 
		 VALUES ($1, $2, $3, $4, NOW()) 
		 ON CONFLICT (team_id) DO UPDATE 
		 SET group_id = $2, admin_api_key = $3, group_api_key = $4, last_updated_timestamp = NOW()`,
		vkChannel.TeamID, vkChannel.GroupID, vkChannel.AdminAPIKey, vkChannel.GroupAPIKey,
	)
	return err
}
func (t *Team) GetVKCredsByTeamID(teamId int) (*entity.VKChannel, error) {
	var vkChannel entity.VKChannel

	err := t.db.Get(
		&vkChannel,
		`SELECT id, team_id, group_id, admin_api_key, group_api_key, last_updated_timestamp 
		 FROM channel_vk 
		 WHERE team_id = $1`,
		teamId,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repo.ErrTGChannelNotFound
		}
		return nil, err
	}

	return &vkChannel, nil
}
