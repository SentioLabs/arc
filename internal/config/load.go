package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// DefaultPath returns ~/.arc/config.toml.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".arc", "config.toml")
}

// LegacyJSONPath returns ~/.arc/cli-config.json (the pre-TOML location).
func LegacyJSONPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".arc", "cli-config.json")
}

// expandHome expands a leading ~ to the user's home directory.
func expandHome(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~"))
}

// Load reads the TOML config from path. If path is missing but the legacy
// cli-config.json exists in the same directory, Load migrates JSON → TOML,
// renames the legacy file to *.bak, and prints a one-line stderr notice.
// If neither exists, Load writes Default() to path and returns it.
// Missing fields are filled from Default(). The returned Config is
// validated; an invalid file returns a non-nil error.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath()
	}

	if _, err := os.Stat(path); err == nil {
		cfg := Default()
		if _, err := toml.DecodeFile(path, cfg); err != nil {
			return nil, fmt.Errorf("decode toml %s: %w", path, err)
		}
		cfg.Server.DBPath = expandHome(cfg.Server.DBPath)
		if err := Validate(cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	// No TOML — try legacy JSON in the same directory.
	legacy := filepath.Join(filepath.Dir(path), "cli-config.json")
	if _, err := os.Stat(legacy); err == nil {
		cfg, err := migrateLegacyJSON(legacy)
		if err != nil {
			return nil, err
		}
		cfg.Server.DBPath = expandHome(cfg.Server.DBPath)
		if err := Save(path, cfg); err != nil {
			return nil, err
		}
		backup := legacy + ".bak"
		if err := os.Rename(legacy, backup); err != nil {
			return nil, fmt.Errorf("backup legacy: %w", err)
		}
		fmt.Fprintf(os.Stderr, "migrated %s → %s (backup: %s)\n", legacy, path, filepath.Base(backup))
		return cfg, nil
	}

	// No config anywhere — write defaults.
	cfg := Default()
	cfg.Server.DBPath = expandHome(cfg.Server.DBPath)
	if err := Save(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
