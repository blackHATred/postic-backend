package cockroach

import (
	"github.com/jmoiron/sqlx"
	"postic-backend/internal/repo"
	"time"
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
