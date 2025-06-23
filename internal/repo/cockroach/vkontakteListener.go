package cockroach

import (
	"postic-backend/internal/repo"
	"time"

	"github.com/jmoiron/sqlx"
)

type VkontakteListener struct {
	db *sqlx.DB
}

func NewVkontakteListener(db *sqlx.DB) repo.VkontakteListener {
	return &VkontakteListener{db: db}
}

func (v *VkontakteListener) GetUnwatchedGroups(duration time.Duration) ([]int, error) {
	query := `
SELECT team_id
FROM channel_vk
WHERE last_updated_timestamp < NOW() - $1::INTERVAL
`
	rows, err := v.db.Queryx(query, duration.Seconds())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var groups []int
	for rows.Next() {
		var groupID int
		if err := rows.Scan(&groupID); err != nil {
			return nil, err
		}
		groups = append(groups, groupID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return groups, nil
}

func (v *VkontakteListener) UpdateGroupLastUpdate(teamID int) error {
	query := `
UPDATE channel_vk
SET last_updated_timestamp = NOW()
WHERE team_id = $1
`
	_, err := v.db.Exec(query, teamID)
	if err != nil {
		return err
	}
	return nil
}

func (v *VkontakteListener) GetLastEventTS(teamID int) (string, error) {
	var lastEventTS string
	query := `
SELECT last_event_ts
FROM channel_vk
WHERE team_id = $1
`
	err := v.db.QueryRow(query, teamID).Scan(&lastEventTS)
	if err != nil {
		return "0", err
	}
	return lastEventTS, nil
}

func (v *VkontakteListener) SetLastEventTS(teamID int, ts string) error {
	query := `
UPDATE channel_vk
SET last_event_ts = $1
WHERE team_id = $2
`
	_, err := v.db.Exec(query, ts, teamID)
	if err != nil {
		return err
	}
	return nil
}
