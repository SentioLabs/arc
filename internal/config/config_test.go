package config_test

import (
	"testing"

	"github.com/sentiolabs/arc/internal/config"
)

// --- Contract assertions ---
// These verify the design spec. Do NOT modify without updating the approved plan.
var (
	_ config.CLIConfig     = config.Config{}.CLI
	_ config.ServerConfig  = config.Config{}.Server
	_ config.ShareConfig   = config.Config{}.Share
	_ config.UpdatesConfig = config.Config{}.Updates
)

var _ interface {
	Load() *config.Config
	Swap(*config.Config) *config.Config
} = (*config.Store)(nil)

func TestDefaultIsUsable(t *testing.T) {
	cfg := config.Default()
	if cfg.CLI.Server == "" || cfg.Server.Port == 0 {
		t.Fatal("Default() returned zero values for required fields")
	}
	if cfg.Updates.Channel != "stable" {
		t.Fatalf("Default channel = %q, want stable", cfg.Updates.Channel)
	}
	if cfg.Share.Server == "" {
		t.Fatal("Default share.server is empty")
	}
}

func TestRequiresRestartContainsServerKeys(t *testing.T) {
	got := config.RequiresRestart()
	want := map[string]bool{"server.port": true, "server.db_path": true}
	if len(got) != len(want) {
		t.Fatalf("RequiresRestart() = %v, want keys %v", got, want)
	}
	for _, k := range got {
		if !want[k] {
			t.Errorf("unexpected key %q in RequiresRestart()", k)
		}
	}
}

func TestStoreSwap(t *testing.T) {
	a := config.Default()
	b := &config.Config{Updates: config.UpdatesConfig{Channel: "rc"}}
	s := config.NewStore(a)
	if got := s.Load(); got != a {
		t.Fatalf("Load() before swap = %p, want %p", got, a)
	}
	prev := s.Swap(b)
	if prev != a {
		t.Fatalf("Swap returned %p, want %p", prev, a)
	}
	if got := s.Load(); got != b {
		t.Fatalf("Load() after swap = %p, want %p", got, b)
	}
}
