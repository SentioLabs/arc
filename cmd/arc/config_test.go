package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cfgpkg "github.com/sentiolabs/arc/internal/config"
)

func TestConfigSetGetRoundTrip(t *testing.T) {
	dir := t.TempDir()
	configPath = filepath.Join(dir, "config.toml")
	defer func() { configPath = "" }()

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if err := setKey(cfg, "cli.server", "http://example.com:9000"); err != nil {
		t.Fatalf("setKey: %v", err)
	}
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}

	got, err := loadConfig()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.CLI.Server != "http://example.com:9000" {
		t.Errorf("cli.server = %q", got.CLI.Server)
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
		serverDBPathKey,
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
	if got != serverDBPathKey {
		t.Errorf("got %q, want %s", got, serverDBPathKey)
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

// projectPlansServer starts an httptest server that serves a single project
// (id "proj-test", name "My Project", prefix "mp") plus its per-project
// config rows. configRows may be nil/empty to simulate an unreachable or
// empty config layer (served as 404, tolerated by resolvePlansForProject).
func projectPlansServer(t *testing.T, configRows map[string]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/proj-test", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id": "proj-test", "name": "My Project", "prefix": "mp",
		})
	})
	mux.HandleFunc("/api/v1/projects/proj-test/config", func(w http.ResponseWriter, r *http.Request) {
		if configRows == nil {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"config": configRows})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// withResolvedTestGlobals points the CLI's global flags at ts and project
// "proj-test", restoring the previous values on cleanup.
func withResolvedTestGlobals(t *testing.T, ts *httptest.Server) {
	t.Helper()
	origServerURL, origProjectID, origOutputJSON := serverURL, projectID, outputJSON
	serverURL, projectID, outputJSON = ts.URL, "proj-test", true
	t.Cleanup(func() { serverURL, projectID, outputJSON = origServerURL, origProjectID, origOutputJSON })
}

// TestRunConfigGetResolvedDegradedPathDefaultDir covers the case with no
// per-project override (config rows unreachable): "config get plans.dir
// --resolved" must still fall back to the global/default plans.dir, since
// resolvePlansForProject tolerates the per-project config fetch failing.
func TestRunConfigGetResolvedDegradedPathDefaultDir(t *testing.T) {
	loadConfigForTest(t)
	ts := projectPlansServer(t, nil)
	withResolvedTestGlobals(t, ts)

	out := captureStdout(t, func() {
		err := runConfigGetResolved()
		if err != nil {
			t.Fatalf("runConfigGetResolved: %v", err)
		}
	})

	var result map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("unmarshal output %q: %v", out, err)
	}
	dir := result[plansDirKey]
	if !strings.HasSuffix(dir, filepath.Join("docs", "plans")) {
		t.Errorf("expected dir ending in docs/plans, got %q", dir)
	}
}

// TestRunConfigGetResolvedReflectsPerProjectOverride is the regression test
// for arc-0ek8.00g6ap: "config get plans.dir --resolved" must agree with
// resolvePlansForProject (the same resolver "arc which" and "arc project
// plans get" use), including any per-project override — not just the
// global-layer plans.dir.
func TestRunConfigGetResolvedReflectsPerProjectOverride(t *testing.T) {
	loadConfigForTest(t)
	override := filepath.Join(t.TempDir(), "vault-override")
	ts := projectPlansServer(t, map[string]string{
		cfgpkg.ProjectPlansDirKey:  override,
		cfgpkg.ProjectPlansTypeKey: cfgpkg.PlansTypeObsidian,
	})
	withResolvedTestGlobals(t, ts)

	out := captureStdout(t, func() {
		err := runConfigGetResolved()
		if err != nil {
			t.Fatalf("runConfigGetResolved: %v", err)
		}
	})

	var result map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("unmarshal output %q: %v", out, err)
	}
	got := result[plansDirKey]
	if got != override {
		t.Errorf("config get plans.dir --resolved = %q, want override %q", got, override)
	}

	// The same resolver backing "arc which" / "arc project plans get" must
	// agree exactly.
	want, _, source, err := resolvePlansForProject("proj-test", "My Project", "mp")
	if err != nil {
		t.Fatalf("resolvePlansForProject: %v", err)
	}
	if got != want {
		t.Errorf("config get plans.dir --resolved (%q) diverges from resolvePlansForProject (%q)", got, want)
	}
	if source != cfgpkg.PlansSourceProject {
		t.Errorf("expected source %q, got %q", cfgpkg.PlansSourceProject, source)
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
