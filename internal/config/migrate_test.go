package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/arc/internal/config"
)

func TestLoadMigratesLegacyJSON(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "cli-config.json")
	if err := os.WriteFile(legacy, []byte(`{
		"server_url": "http://example:1234",
		"channel": "rc",
		"share_author": "Grace",
		"share_server": "https://share.example"
	}`), 0o600); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	path := filepath.Join(dir, "config.toml")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CLI.Server != "http://example:1234" {
		t.Errorf("cli.server = %q", cfg.CLI.Server)
	}
	if cfg.Updates.Channel != "rc" {
		t.Errorf("updates.channel = %q", cfg.Updates.Channel)
	}
	if cfg.Share.Author != "Grace" {
		t.Errorf("share.author = %q", cfg.Share.Author)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy file still present: %v", err)
	}
	if _, err := os.Stat(legacy + ".bak"); err != nil {
		t.Errorf("legacy backup missing: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("toml file missing: %v", err)
	}
}
