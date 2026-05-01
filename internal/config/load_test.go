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
