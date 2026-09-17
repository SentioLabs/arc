package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

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

func TestLoadReadOnlyPreservesMissingAndLegacyConfig(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "legacy"}[legacy], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "nested", "config.toml")
			original := []byte(`{"server_url":"http://localhost:12345","channel":"stable"}`)
			legacyPath := filepath.Join(filepath.Dir(path), "cli-config.json")
			if legacy {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
				require.NoError(t, os.WriteFile(legacyPath, original, 0o600))
			}
			cfg, err := config.LoadReadOnly(path)
			require.NoError(t, err)
			if legacy {
				require.Equal(t, "http://localhost:12345", cfg.CLI.Server)
			} else {
				require.Equal(t, config.Default(), cfg)
			}
			_, err = os.Stat(path)
			require.ErrorIs(t, err, os.ErrNotExist)
			_, err = os.Stat(legacyPath + ".bak")
			require.ErrorIs(t, err, os.ErrNotExist)
			if legacy {
				contents, err := os.ReadFile(legacyPath)
				require.NoError(t, err)
				require.Equal(t, original, contents)
			} else {
				_, err = os.Stat(filepath.Dir(path))
				require.ErrorIs(t, err, os.ErrNotExist)
			}
		})
	}
}
