package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	cfgpkg "github.com/sentiolabs/arc/internal/config"
)

const dbPathKey = "server.db_path"

func TestConfigSetGetRoundTrip(t *testing.T) {
	dir := t.TempDir()
	configPath = filepath.Join(dir, "config.toml")
	defer func() { configPath = "" }()

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if err := setKey(cfg, "share.author", "Ada"); err != nil {
		t.Fatalf("setKey: %v", err)
	}
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}

	got, err := loadConfig()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Share.Author != "Ada" {
		t.Errorf("share.author = %q", got.Share.Author)
	}
}

func TestNormalizeKeyUnknownReturnsHint(t *testing.T) {
	_, err := normalizeKey("server_url")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "did you mean") {
		t.Errorf("error missing hint: %v", err)
	}
}

func TestNormalizeKeyLegacyAliases(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"server_url", "cli.server"},
		{"share_author", "share.author"},
		{"share_server", "share.server"},
		{"channel", "updates.channel"},
	}
	for _, tc := range cases {
		_, err := normalizeKey(tc.input)
		if err == nil {
			t.Errorf("normalizeKey(%q): expected error, got nil", tc.input)
			continue
		}
		if !strings.Contains(err.Error(), "did you mean "+tc.want) {
			t.Errorf("normalizeKey(%q): expected hint %q in error, got: %v", tc.input, tc.want, err)
		}
	}
}

func TestSetKeyRejectsBadPort(t *testing.T) {
	cfg, _ := loadConfigForTest(t)
	if err := setKey(cfg, "server.port", "abc"); err == nil {
		t.Fatal("expected parse error")
	}
	if err := setKey(cfg, "server.port", "70000"); err == nil {
		t.Fatal("expected validation error")
	}
}

func loadConfigForTest(t *testing.T) (*cfgpkg.Config, string) {
	t.Helper()
	dir := t.TempDir()
	configPath = filepath.Join(dir, "config.toml")
	t.Cleanup(func() { configPath = ""; _ = os.RemoveAll(dir) })
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	return cfg, configPath
}

func TestNormalizeKeyValid(t *testing.T) {
	validKeys := []string{
		"cli.server",
		"server.port",
		dbPathKey,
		"share.author",
		"share.server",
		"updates.channel",
	}
	for _, k := range validKeys {
		got, err := normalizeKey(k)
		if err != nil {
			t.Errorf("normalizeKey(%q) returned error: %v", k, err)
		}
		if got != k {
			t.Errorf("normalizeKey(%q) = %q, want %q", k, got, k)
		}
	}
}

func TestNormalizeKeyNormalizes(t *testing.T) {
	// Should normalize dashes to underscores
	got, err := normalizeKey("server.db-path")
	if err != nil {
		t.Errorf("normalizeKey(server.db-path): %v", err)
	}
	if got != dbPathKey {
		t.Errorf("got %q, want %s", got, dbPathKey)
	}
}

func TestGetKeyReturnsDefaults(t *testing.T) {
	cfg := cfgpkg.Default()
	val := getKey(cfg, "cli.server")
	if val != "http://localhost:7432" {
		t.Errorf("cli.server default = %q", val)
	}
	val = getKey(cfg, "server.port")
	if val != "7432" {
		t.Errorf("server.port default = %q", val)
	}
}

func TestSetKeyCliServer(t *testing.T) {
	cfg, _ := loadConfigForTest(t)
	if err := setKey(cfg, "cli.server", "http://example.com:9000"); err != nil {
		t.Fatalf("setKey: %v", err)
	}
	if cfg.CLI.Server != "http://example.com:9000" {
		t.Errorf("cli.server = %q", cfg.CLI.Server)
	}
}

func TestLevenshtein(t *testing.T) {
	// Exact match
	if d := levenshtein("foo", "foo"); d != 0 {
		t.Errorf("levenshtein(foo, foo) = %d, want 0", d)
	}
	// Empty
	if d := levenshtein("", "abc"); d != 3 {
		t.Errorf("levenshtein('', abc) = %d, want 3", d)
	}
	// One edit
	if d := levenshtein("cat", "cut"); d != 1 {
		t.Errorf("levenshtein(cat, cut) = %d, want 1", d)
	}
}
