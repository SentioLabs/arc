package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// configFilePerm is the permission for the written config file (owner read/write only).
// Using 0600 ensures only the owner can read sensitive settings.
const configFilePerm = 0o600

// configDirPerm is the permission for the config directory.
const configDirPerm = 0o700

// Save atomically writes cfg to path as TOML with 0600 permissions.
// The write is atomic: it creates a temp file, fsyncs, then renames over path.
// The config is validated before writing; invalid configs return an error.
func Save(path string, cfg *Config) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, configDirPerm); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".config.toml.*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	success := false
	// Clean up the temp file if we exit without renaming it into place.
	defer func() {
		if !success {
			_ = os.Remove(tmpName)
		}
	}()

	if err := toml.NewEncoder(tmp).Encode(cfg); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encode toml: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("fsync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, configFilePerm); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	success = true
	return nil
}
