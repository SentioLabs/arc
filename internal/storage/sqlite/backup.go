// Package sqlite implements the storage interface using SQLite.
// This file provides database backup functionality for the CLI.
package sqlite

import (
	"compress/gzip"
	"database/sql"
	"fmt"
	"io"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

// BackupResult holds metadata about a completed backup.
type BackupResult struct {
	Path         string // Path to the backup file
	OriginalSize int64  // Size of the original database file in bytes
	BackupSize   int64  // Size of the compressed backup in bytes
}

// BackupDatabase creates a timestamped, gzipped backup of a SQLite database file.
// It opens its own connection to perform the WAL checkpoint, so no existing
// connection is needed. Use BackupDatabaseConn if you already have a connection.
func BackupDatabase(dbPath string) (*BackupResult, error) {
	sqlDB, err := sql.Open("sqlite", dbPath+"?_busy_timeout=5000&_journal_mode=WAL&mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	defer sqlDB.Close()

	return BackupDatabaseConn(sqlDB, dbPath)
}

// BackupDatabaseConn creates a timestamped, gzipped backup of a SQLite database
// using an existing connection. The connection is used for the WAL checkpoint
// but is NOT closed by this function.
//
// The backup file is written to the same directory as the database with the
// format: <dbfile>.<YYYYMMDD_HHMMSS>.gz
//
// Returns nil result (no error) if the database file doesn't exist or is empty.
func BackupDatabaseConn(sqlDB *sql.DB, dbPath string) (*BackupResult, error) {
	info, err := os.Stat(dbPath)
	if err != nil || info.Size() == 0 {
		return nil, nil //nolint:nilerr // no file to back up
	}

	originalSize := info.Size()

	// Flush WAL into the main DB file so the copy is self-contained
	if _, err := sqlDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return nil, fmt.Errorf("checkpoint WAL: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s.%s.gz", dbPath, timestamp)

	if err := gzipCopy(dbPath, backupPath); err != nil {
		return nil, fmt.Errorf("create backup: %w", err)
	}

	backupInfo, err := os.Stat(backupPath)
	if err != nil {
		return nil, fmt.Errorf("stat backup: %w", err)
	}

	return &BackupResult{
		Path:         backupPath,
		OriginalSize: originalSize,
		BackupSize:   backupInfo.Size(),
	}, nil
}

// gzipCopy reads src and writes a gzip-compressed copy to dst.
func gzipCopy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		// Clean up partial file on error
		if err != nil {
			_ = out.Close()
			_ = os.Remove(dst)
		}
	}()

	gz, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		return err
	}

	if _, err = io.Copy(gz, in); err != nil {
		return err
	}

	if err = gz.Close(); err != nil {
		return err
	}

	return out.Close()
}
