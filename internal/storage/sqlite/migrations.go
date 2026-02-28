// Package sqlite implements the storage interface using SQLite.
// This file handles database schema migrations using goose.
package sqlite

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

// migrations embeds all SQL migration files from the migrations directory.
//
//go:embed migrations/*.sql
var migrations embed.FS

// init configures goose to use the embedded filesystem for migration files.
func init() {
	goose.SetBaseFS(migrations)
}

// RunMigrations applies all pending database migrations.
func RunMigrations(db *sql.DB) error {
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	return goose.Up(db, "migrations")
}

// MigrationVersion returns the current migration version.
func MigrationVersion(db *sql.DB) (int64, error) {
	if err := goose.SetDialect("sqlite3"); err != nil {
		return 0, err
	}
	return goose.GetDBVersion(db)
}
