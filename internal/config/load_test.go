package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/arc/internal/config"
)

const tildeDBPath = "~/.arc/data.db"

func TestLoadCreatesDefaultWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CLI.Server == "" {
		t.Error("default CLI.Server was empty")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Load did not write default file: %v", err)
	}
}

func TestLoadSavePreservesTildeInDBPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := config.Default()
	cfg.Server.DBPath = tildeDBPath
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Server.DBPath != tildeDBPath {
		t.Errorf("db_path = %q, want %q (tilde corrupted by load)", got.Server.DBPath, tildeDBPath)
	}
	if err := config.Save(path, got); err != nil {
		t.Fatalf("Save (round 2): %v", err)
	}
	got2, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load (round 2): %v", err)
	}
	if got2.Server.DBPath != tildeDBPath {
		t.Errorf("db_path after round-trip = %q, want preserved tilde", got2.Server.DBPath)
	}
}
