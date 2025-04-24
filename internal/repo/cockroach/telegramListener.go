package cockroach

import (
	"github.com/jmoiron/sqlx"
	"postic-backend/internal/repo"
)

type TelegramListener struct {
	db *sqlx.DB
}

func NewTelegramListener(db *sqlx.DB) repo.TelegramListener {
	return &TelegramListener{db: db}
}

func (l *TelegramListener) GetLastUpdate() (int, error) {
	// Пример запроса к базе данных для получения последнего обновления
	var lastUpdate int
	err := l.db.QueryRow("SELECT last_update_id FROM tg_bot_state WHERE id = 1").Scan(&lastUpdate)
	if err != nil {
		return 0, err
	}
	return lastUpdate, nil
}

func (l *TelegramListener) SetLastUpdate(i int) error {
	// Пример запроса к базе данных для установки последнего обновления
	_, err := l.db.Exec("UPDATE tg_bot_state SET last_update_id = $1 WHERE id = 1", i)
	if err != nil {
		return err
	}
	return nil
}
