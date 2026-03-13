// Package sqlite implements the storage interface using SQLite.
// This file handles database schema migrations using goose.
package sqlite

import (
	"database/sql"
	"embed"
	"fmt"
	"io"
	"os"

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

// backupForMigration creates a copy of the database file for safe migration rollback.
// Checkpoints the WAL first to ensure the backup is self-contained.
// Returns the backup path, or "" if no backup was needed (file doesn't exist or is empty).
func backupForMigration(sqlDB *sql.DB, dbPath string) (string, error) {
	info, statErr := os.Stat(dbPath)
	if statErr != nil || info.Size() == 0 {
		return "", nil //nolint:nilerr // stat error means no file to back up
	}

	// Flush WAL into the main DB file so the copy is self-contained
	if _, err := sqlDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return "", fmt.Errorf("checkpoint WAL: %w", err)
	}

	backupPath := dbPath + ".pre-migration-backup"
	if err := copyFile(dbPath, backupPath); err != nil {
		return "", fmt.Errorf("copy database: %w", err)
	}

	return backupPath, nil
}

// restoreBackup overwrites the database file with the backup and cleans up
// stale WAL/SHM files left behind by the failed migration.
func restoreBackup(dbPath, backupPath string) error {
	if err := copyFile(backupPath, dbPath); err != nil {
		return fmt.Errorf("restore backup: %w", err)
	}

	// Remove stale WAL/SHM files from the failed migration
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")

	_ = os.Remove(backupPath)
	return nil
}

// copyFile copies src to dst, creating or truncating dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Close()
}
