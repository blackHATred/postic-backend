package cockroach

import (
	"database/sql"
	"errors"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"postic-backend/internal/repo"
)

type Team struct {
	db *sqlx.DB
}

func NewTeam(db *sqlx.DB) repo.Team {
	return &Team{db: db}
}

func (t *Team) GetTGChannelByTeamID(teamId int) (int, error) {
	var channelId int
	err := t.db.Get(&channelId, "SELECT channel_id FROM channel_tg WHERE team_id = $1", teamId)
	if err != nil {
		return 0, err
	}
	return channelId, nil
}

func (t *Team) GetUserPermissionsByTeamID(teamId int, userId int) ([]repo.UserTeamRole, error) {
	var roles []string
	query := "SELECT roles FROM team_user_role WHERE team_id = $1 AND user_id = $2"
	if err := t.db.QueryRow(query, teamId, userId).Scan(pq.Array(&roles)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []repo.UserTeamRole{}, nil
		}
		return nil, err
	}
	var userRoles []repo.UserTeamRole
	for _, role := range roles {
		userRoles = append(userRoles, repo.UserTeamRole(role))
	}
	return userRoles, nil
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
	if err != nil {
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
