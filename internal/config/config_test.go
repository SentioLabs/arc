package config_test

import (
	"path/filepath"
	"testing"

	"github.com/sentiolabs/arc/internal/config"
)

// --- Contract assertions ---
// These verify the design spec. Do NOT modify without updating the approved plan.
var (
	_ config.CLIConfig     = config.Config{}.CLI
	_ config.ServerConfig  = config.Config{}.Server
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
}

func TestRequiresRestartContainsServerKeys(t *testing.T) {
	got := config.RequiresRestart()
	want := map[string]bool{"server.port": true, "server.db_path": true, "server.plans_dir": true}
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

func TestServerPlansRootIndependentAndRoundTrips(t *testing.T) {
	cfg := config.Default()
	if cfg.Server.PlansDir != "~/.arc/plans" {
		t.Fatalf("server default = %q", cfg.Server.PlansDir)
	}
	cfg.Plans.Dir = "/unrelated/client/drafts"
	cfg.Plans.Type = config.PlansTypeObsidian
	if cfg.Server.PlansDir != "~/.arc/plans" {
		t.Fatal("client plans changed server root")
	}
	cfg.Server.PlansDir = "/operator/content"
	path := t.TempDir() + "/config.toml"
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Server.PlansDir != cfg.Server.PlansDir {
		t.Fatalf("got %q", loaded.Server.PlansDir)
	}
}

func TestResolvedServerPlansDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := config.Default()
	got, err := cfg.Server.ResolvedPlansDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(home, ".arc", "plans") {
		t.Fatalf("resolved root = %q", got)
	}
	if cfg.Server.PlansDir != "~/.arc/plans" {
		t.Fatal("resolution mutated stored config")
	}
	cfg.Server.PlansDir = "relative-plans"
	got, err = cfg.Server.ResolvedPlansDir()
	if err != nil || !filepath.IsAbs(got) {
		t.Fatalf("root = %q, error = %v", got, err)
	}
	cfg.Server.PlansDir = ""
	if _, err := cfg.Server.ResolvedPlansDir(); err == nil {
		t.Fatal("empty configured root must fail")
	}
}
