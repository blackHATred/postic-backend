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

func (t *Team) GetTGChannelByTeamID(teamId int) (int, int, error) {
	var channelId, discussionId sql.NullInt64
	err := t.db.QueryRow(
		"SELECT channel_id, discussion_id FROM channel_tg WHERE team_id = $1",
		teamId,
	).Scan(&channelId, &discussionId)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	// Возвращаем 0, если поле NULL
	var chId, discId int
	if channelId.Valid {
		chId = int(channelId.Int64)
	}
	if discussionId.Valid {
		discId = int(discussionId.Int64)
	}

	return chId, discId, nil
}

func (t *Team) GetTeamIDBySecret(secret string) (int, error) {
	var teamID int
	err := t.db.Get(&teamID, "SELECT id FROM team WHERE secret = $1", secret)
	if err != nil {
		return 0, err
	}
	return teamID, nil
}

func (t *Team) PutTGChannel(teamId int, channelId int, discussionId int) error {
	_, err := t.db.Exec(
		"INSERT INTO channel_tg (team_id, channel_id, discussion_id) VALUES ($1, $2, $3) ON CONFLICT (team_id) DO UPDATE SET channel_id = $2, discussion_id = $3",
		teamId, channelId, discussionId,
	)
	return err
}

func (t *Team) GetTGChannelByDiscussionId(discussionId int) (int, error) {
	var channelId int
	err := t.db.Get(&channelId, "SELECT channel_id FROM channel_tg WHERE discussion_id = $1", discussionId)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, repo.ErrTGChannelNotFound
	case err != nil:
		return 0, err
	}
	return channelId, nil
}

func (t *Team) GetTeamIDByPostUnionID(postUnionID int) (int, error) {
	var teamId int
	err := t.db.Get(&teamId, "SELECT team_id FROM post_union WHERE id = $1", postUnionID)
	if err != nil {
		return 0, err
	}
	return teamId, nil
}

func (t *Team) PutVKGroup(teamId int, groupId int, adminApiKey string, groupApiKey string) error {
	_, err := t.db.Exec(
		"INSERT INTO channel_vk (team_id, group_id, admin_api_key, group_api_key) VALUES ($1, $2, $3, $4) "+
			"ON CONFLICT (team_id) DO UPDATE SET group_id = $2, admin_api_key = $3, group_api_key = $4, last_updated_timestamp = NOW()",
		teamId, groupId, adminApiKey, groupApiKey,
	)
	return err
}

func (t *Team) GetVKCredsByTeamID(teamId int) (int, string, string, error) {
	var groupId int
	var adminApiKey, groupApiKey string

	err := t.db.QueryRow(
		"SELECT group_id, admin_api_key, group_api_key FROM channel_vk WHERE team_id = $1",
		teamId,
	).Scan(&groupId, &adminApiKey, &groupApiKey)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, "", "", repo.ErrTGChannelNotFound
		}
		return 0, "", "", err
	}

	return groupId, adminApiKey, groupApiKey, nil
}
