package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// DefaultPath returns ~/.arc/config.toml.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Errorf("cannot determine home directory: %w", err))
	}
	return filepath.Join(home, ".arc", "config.toml")
}

// LegacyJSONPath returns ~/.arc/cli-config.json (the pre-TOML location).
func LegacyJSONPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Errorf("cannot determine home directory: %w", err))
	}
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
		if err := Validate(cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	// No TOML — try legacy JSON in the same directory.
	legacy := filepath.Join(filepath.Dir(path), "cli-config.json")
	if _, err := os.Stat(legacy); err == nil {
		cfg, err := migrateLegacyJSON(legacy)
		if err != nil {
			return nil, err
		}
		// Fix 1: rename legacy → .bak FIRST, then save TOML.
		// If rename fails, return error before touching the TOML path.
		backup := legacy + ".bak"
		if err := os.Rename(legacy, backup); err != nil {
			return nil, fmt.Errorf("backup legacy: %w", err)
		}
		if err := Save(path, cfg); err != nil {
			return nil, err
		}
		fmt.Fprintf(os.Stderr, "migrated %s → %s (backup: %s)\n", legacy, path, filepath.Base(backup))
		return cfg, nil
	}

	// No config anywhere — write defaults.
	cfg := Default()
	if err := Save(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
