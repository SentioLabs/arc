package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	_ "modernc.org/sqlite"

	"github.com/sentiolabs/arc/internal/paste"
	pastesqlite "github.com/sentiolabs/arc/internal/paste/sqlite"
	"github.com/sentiolabs/arc/internal/sharesconfig"
)

func startTestPasteServer(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := pastesqlite.Apply(context.Background(), db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	e := echo.New()
	paste.NewHandlers(pastesqlite.New(db)).Register(e.Group("/api/paste"))
	return httptest.NewServer(e)
}

func TestShareCreateRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	srv := startTestPasteServer(t)
	defer srv.Close()

	plan := filepath.Join(t.TempDir(), "plan.md")
	_ = os.WriteFile(plan, []byte("# Hello\n\nBody."), 0o644)

	shareCreateServer = srv.URL
	if err := runShareCreate(shareCreateCmd, []string{plan}); err != nil {
		t.Fatalf("runShareCreate: %v", err)
	}

	f, _ := sharesconfig.Load()
	if len(f.Shares) != 1 {
		t.Fatalf("expected 1 share recorded, got %d", len(f.Shares))
	}
	s := f.Shares[0]
	if s.URL != srv.URL {
		t.Errorf("URL mismatch: %s vs %s", s.URL, srv.URL)
	}
	if s.EditToken == "" || s.KeyB64Url == "" {
		t.Errorf("missing edit_token or key: %+v", s)
	}
}

func TestResolveShareRefFromURL(t *testing.T) {
	id, server, key, err := resolveShareRef("https://arcplanner.sentiolabs.io/share/abc12345#k=AAAA")
	if err != nil {
		t.Fatal(err)
	}
	if id != "abc12345" || server != "https://arcplanner.sentiolabs.io" || len(key) == 0 {
		t.Errorf("bad parse: id=%s server=%s key=%v", id, server, key)
	}
}

func TestRunShareCommentsRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	srv := startTestPasteServer(t)
	defer srv.Close()

	// Create a plan
	plan := filepath.Join(t.TempDir(), "p.md")
	_ = os.WriteFile(plan, []byte("# P"), 0o644)
	shareCreateServer = srv.URL
	_ = runShareCreate(shareCreateCmd, []string{plan})

	f, _ := sharesconfig.Load()
	s := f.Shares[0]
	keyBytes := mustDecodeKey(t, s.KeyB64Url)

	// Manually post a comment event.
	c := map[string]any{
		"kind": "comment", "id": "c1", "author_name": "Alice", "comment_type": "comment",
		"body": "looks good", "anchor": map[string]any{"line_start": 1, "line_end": 1, "quoted_text": "P"},
		"created_at": "2026-04-29T00:00:00Z",
	}
	blob, iv, _ := paste.EncryptJSON(c, keyBytes)
	body, _ := json.Marshal(paste.AppendEventRequest{Blob: blob, IV: iv})
	if err := postRaw(t, srv.URL+"/api/paste/"+s.ID+"/blobs", body); err != nil {
		t.Fatal(err)
	}

	// Capture stdout while running comments
	out := captureStdout(t, func() { _ = runShareComments(shareCommentsCmd, []string{s.ID}) })
	if !strings.Contains(out, "Alice") || !strings.Contains(out, "looks good") {
		t.Errorf("expected Alice/looks good in output, got: %s", out)
	}
}

// TestRunShareCommentsAppliesEdits verifies that an `edit` event from the
// comment's original author rewrites the body shown by `arc share comments`.
// This locks in CLI parity with the SPA's replay logic.
func TestRunShareCommentsAppliesEdits(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	srv := startTestPasteServer(t)
	defer srv.Close()

	plan := filepath.Join(t.TempDir(), "p.md")
	_ = os.WriteFile(plan, []byte("# P"), 0o644)
	shareCreateServer = srv.URL
	_ = runShareCreate(shareCreateCmd, []string{plan})

	f, _ := sharesconfig.Load()
	s := f.Shares[0]
	keyBytes := mustDecodeKey(t, s.KeyB64Url)

	postEv := func(t *testing.T, payload map[string]any) {
		t.Helper()
		blob, iv, err := paste.EncryptJSON(payload, keyBytes)
		if err != nil {
			t.Fatalf("encrypt: %v", err)
		}
		body, _ := json.Marshal(paste.AppendEventRequest{Blob: blob, IV: iv})
		if err := postRaw(t, srv.URL+"/api/paste/"+s.ID+"/blobs", body); err != nil {
			t.Fatal(err)
		}
	}

	// 1) Steve posts a thin "expand this more" comment.
	postEv(t, map[string]any{
		"kind": "comment", "id": "c1", "author_name": "Steve", "comment_type": "comment",
		"body":       "expand this more",
		"anchor":     map[string]any{"line_start": 1, "line_end": 1, "quoted_text": "P"},
		"created_at": "2026-04-29T00:00:00Z",
	})

	// 2) Steve revises it with a fully-formed thought.
	postEv(t, map[string]any{
		"kind": "edit", "id": "e1", "comment_id": "c1", "author_name": "Steve",
		"body":       "the goal section should mention the success criteria for ‘validated’",
		"created_at": "2026-04-29T00:05:00Z",
	})

	// 3) Mallory tries to forge an edit pretending to be Steve. (Wrong author_name
	//    on the edit event; replay must drop it.)
	postEv(t, map[string]any{
		"kind": "edit", "id": "e2", "comment_id": "c1", "author_name": "Mallory",
		"body":       "MALICIOUS REWRITE",
		"created_at": "2026-04-29T00:06:00Z",
	})

	out := captureStdout(t, func() { _ = runShareComments(shareCommentsCmd, []string{s.ID}) })

	if !strings.Contains(out, "success criteria") {
		t.Errorf("expected edited body in output; got:\n%s", out)
	}
	if strings.Contains(out, "expand this more") {
		t.Errorf("expected stale body to be replaced; got:\n%s", out)
	}
	if strings.Contains(out, "MALICIOUS REWRITE") {
		t.Errorf("forged edit must be ignored at replay time; got:\n%s", out)
	}
}

// mustDecodeKey decodes a base64url key or fatals the test.
func mustDecodeKey(t *testing.T, b64 string) []byte {
	t.Helper()
	key, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("decode key: %v", err)
	}
	return key
}

// postRaw sends an HTTP POST with a JSON body to url and fails if the status
// is not 2xx.
func postRaw(t *testing.T, url string, body []byte) error {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("postRaw %s: %s: %s", url, resp.Status, b)
	}
	return nil
}

// captureStdout captures writes to os.Stdout during fn and returns the
// captured output as a string.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// contains is a convenience wrapper around strings.Contains.
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestResolveAuthor locks in the resolution precedence:
//
//	flag > config file > env var > git config (lowest)
//
// We isolate from the user's real ~/.arc/cli-config.json by pointing the
// global `configPath` at a temp file, and from the user's git identity by
// running each subtest with $PATH cleared so `git` is unavailable (the
// helper falls back silently to "" on git failure).
func TestResolveAuthor(t *testing.T) {
	// Save & restore globals touched by the helper.
	origConfigPath := configPath
	origEnv := os.Getenv("ARC_SHARE_AUTHOR")
	origPath := os.Getenv("PATH")
	t.Cleanup(func() {
		configPath = origConfigPath
		_ = os.Setenv("ARC_SHARE_AUTHOR", origEnv)
		_ = os.Setenv("PATH", origPath)
	})

	// Strip git from PATH so the lowest tier resolves to "" deterministically.
	_ = os.Setenv("PATH", "")

	writeConfig := func(t *testing.T, author string) {
		t.Helper()
		dir := t.TempDir()
		configPath = filepath.Join(dir, "cli-config.json")
		body := `{"server_url":"http://localhost:7432"`
		if author != "" {
			body += `,"share_author":"` + author + `"`
		}
		body += `}`
		if err := os.WriteFile(configPath, []byte(body), 0600); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}

	t.Run("flag wins over everything", func(t *testing.T) {
		writeConfig(t, "from-config")
		_ = os.Setenv("ARC_SHARE_AUTHOR", "from-env")
		if got := resolveAuthor("from-flag"); got != "from-flag" {
			t.Errorf("flag should win; got %q", got)
		}
	})

	t.Run("config wins over env when no flag", func(t *testing.T) {
		writeConfig(t, "from-config")
		_ = os.Setenv("ARC_SHARE_AUTHOR", "from-env")
		if got := resolveAuthor(""); got != "from-config" {
			t.Errorf("config should win over env; got %q", got)
		}
	})

	t.Run("env wins when config empty", func(t *testing.T) {
		writeConfig(t, "")
		_ = os.Setenv("ARC_SHARE_AUTHOR", "from-env")
		if got := resolveAuthor(""); got != "from-env" {
			t.Errorf("env should win; got %q", got)
		}
	})

	t.Run("flag whitespace is trimmed and treated as empty", func(t *testing.T) {
		writeConfig(t, "from-config")
		_ = os.Setenv("ARC_SHARE_AUTHOR", "")
		if got := resolveAuthor("   "); got != "from-config" {
			t.Errorf("whitespace flag should fall through; got %q", got)
		}
	})

	t.Run("everything empty resolves to empty string", func(t *testing.T) {
		writeConfig(t, "")
		_ = os.Setenv("ARC_SHARE_AUTHOR", "")
		if got := resolveAuthor(""); got != "" {
			t.Errorf("expected empty; got %q", got)
		}
	})
}

// TestResolveServer locks in the share-server precedence:
//
//	--server flag > share_server in ~/.arc/cli-config.json > $ARC_SHARE_SERVER > built-in default
//
// We isolate from the user's real cli-config.json by pointing the global
// `configPath` at a temp file. The `_, kind` return is asserted as
// "shared" everywhere a flag/env/config resolution is expected, since
// shared mode is the only branch that consults these sources.
func TestResolveServer(t *testing.T) {
	const builtinDefault = "https://arcplanner.sentiolabs.io"

	origConfigPath := configPath
	origEnv := os.Getenv("ARC_SHARE_SERVER")
	t.Cleanup(func() {
		configPath = origConfigPath
		_ = os.Setenv("ARC_SHARE_SERVER", origEnv)
	})

	writeConfigShareServer := func(t *testing.T, server string) {
		t.Helper()
		dir := t.TempDir()
		configPath = filepath.Join(dir, "cli-config.json")
		body := `{"server_url":"http://localhost:7432"`
		if server != "" {
			body += `,"share_server":"` + server + `"`
		}
		body += `}`
		if err := os.WriteFile(configPath, []byte(body), 0600); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}

	t.Run("flag wins over everything", func(t *testing.T) {
		writeConfigShareServer(t, "https://from-config.example")
		_ = os.Setenv("ARC_SHARE_SERVER", "https://from-env.example")
		got, kind := resolveServer(false, true, "https://from-flag.example")
		if got != "https://from-flag.example" || kind != "shared" {
			t.Errorf("flag should win; got (%q, %q)", got, kind)
		}
	})

	t.Run("flag wins even without --share", func(t *testing.T) {
		// The override flag is intentionally global — it forces shared mode
		// regardless of which boolean flags are set, mirroring the previous
		// behavior. Locked in here so it doesn't silently drift.
		writeConfigShareServer(t, "https://from-config.example")
		got, kind := resolveServer(true, false, "https://from-flag.example")
		if got != "https://from-flag.example" || kind != "shared" {
			t.Errorf("flag should win even without --share; got (%q, %q)", got, kind)
		}
	})

	t.Run("config wins over env when no flag", func(t *testing.T) {
		writeConfigShareServer(t, "https://from-config.example")
		_ = os.Setenv("ARC_SHARE_SERVER", "https://from-env.example")
		got, kind := resolveServer(false, true, "")
		if got != "https://from-config.example" || kind != "shared" {
			t.Errorf("config should win over env; got (%q, %q)", got, kind)
		}
	})

	t.Run("env wins when config empty", func(t *testing.T) {
		writeConfigShareServer(t, "")
		_ = os.Setenv("ARC_SHARE_SERVER", "https://from-env.example")
		got, kind := resolveServer(false, true, "")
		if got != "https://from-env.example" || kind != "shared" {
			t.Errorf("env should win; got (%q, %q)", got, kind)
		}
	})

	t.Run("falls back to built-in default", func(t *testing.T) {
		writeConfigShareServer(t, "")
		_ = os.Setenv("ARC_SHARE_SERVER", "")
		got, kind := resolveServer(false, true, "")
		if got != builtinDefault || kind != "shared" {
			t.Errorf("expected default; got (%q, %q)", got, kind)
		}
	})

	t.Run("flag whitespace is trimmed and treated as empty", func(t *testing.T) {
		writeConfigShareServer(t, "https://from-config.example")
		_ = os.Setenv("ARC_SHARE_SERVER", "")
		got, kind := resolveServer(false, true, "   ")
		if got != "https://from-config.example" || kind != "shared" {
			t.Errorf("whitespace flag should fall through; got (%q, %q)", got, kind)
		}
	})
}
