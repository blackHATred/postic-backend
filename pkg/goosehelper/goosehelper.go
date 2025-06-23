package goosehelper

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

// MigrateUp выполняет миграции из директории migrationsDir
func MigrateUp(db *sql.DB, migrationsDir string) {
	if err := goose.Up(db, migrationsDir); err != nil {
		log.Fatalf("goose up failed: %v", err)
	}
}
