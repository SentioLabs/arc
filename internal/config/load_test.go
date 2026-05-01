package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesDefaultWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg, err := Load(path)
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
	cfg := Default()
	cfg.Server.DBPath = "~/.arc/data.db"
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Server.DBPath != "~/.arc/data.db" {
		t.Errorf("db_path = %q, want %q (tilde corrupted by load)", got.Server.DBPath, "~/.arc/data.db")
	}
	if err := Save(path, got); err != nil {
		t.Fatalf("Save (round 2): %v", err)
	}
	got2, err := Load(path)
	if err != nil {
		t.Fatalf("Load (round 2): %v", err)
	}
	if got2.Server.DBPath != "~/.arc/data.db" {
		t.Errorf("db_path after round-trip = %q, want preserved tilde", got2.Server.DBPath)
	}
}
