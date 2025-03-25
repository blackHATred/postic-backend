package connector

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func GetCockroachConnector(dsn string) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", dsn) // cockroach работает с драйвером postgres
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(0)
	return db, nil
}
