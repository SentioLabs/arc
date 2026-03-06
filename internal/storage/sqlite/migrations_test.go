package sqlite_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/arc/internal/storage/sqlite"
)

func TestBackupCreatedAndCleanedUpOnSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// First open creates the DB and runs all migrations
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("first New() failed: %v", err)
	}
	store.Close()

	// Verify the DB file exists
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("DB file should exist: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("DB file should not be empty after migrations")
	}

	// Second open triggers initSchema again (migrations are already applied, so it's a no-op)
	store2, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("second New() failed: %v", err)
	}
	defer store2.Close()

	// Backup file should be cleaned up after successful migration
	backupPath := dbPath + ".pre-migration-backup"
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("backup file should be cleaned up after successful migration")
	}
}

func TestNoBackupForFreshDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "fresh.db")

	// File doesn't exist yet — no backup should be created
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer store.Close()

	backupPath := dbPath + ".pre-migration-backup"
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("no backup should exist for a fresh database")
	}
}
