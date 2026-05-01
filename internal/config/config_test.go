package config

import "testing"

// --- Contract assertions ---
// These verify the design spec. Do NOT modify without updating the approved plan.
var (
	_ CLIConfig     = Config{}.CLI
	_ ServerConfig  = Config{}.Server
	_ ShareConfig   = Config{}.Share
	_ UpdatesConfig = Config{}.Updates
)

var _ interface {
	Load() *Config
	Swap(*Config) *Config
} = (*Store)(nil)

func TestDefaultIsUsable(t *testing.T) {
	cfg := Default()
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
	got := RequiresRestart()
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
	a := Default()
	b := &Config{Updates: UpdatesConfig{Channel: "rc"}}
	s := NewStore(a)
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
