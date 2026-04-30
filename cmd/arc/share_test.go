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

// TestRunShareCommentsAppliesPlanAuthorEdits verifies that the plan author
// can edit any reviewer's comment, mirroring the SPA's replay rule. This is
// the "Ben sharpens Steve's 'expand this more' into something useful"
// workflow — comment.author_name stays as the original reviewer.
func TestRunShareCommentsAppliesPlanAuthorEdits(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	srv := startTestPasteServer(t)
	defer srv.Close()

	plan := filepath.Join(t.TempDir(), "p.md")
	_ = os.WriteFile(plan, []byte("# P"), 0o644)
	shareCreateServer = srv.URL
	// Pin the plan author via flag so the replay knows who has author rights.
	shareCreateAuthor = "Ben"
	t.Cleanup(func() { shareCreateAuthor = "" })
	if err := runShareCreate(shareCreateCmd, []string{plan}); err != nil {
		t.Fatal(err)
	}
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

	// Steve leaves a thin comment, then Ben (plan author) refines the body.
	postEv(t, map[string]any{
		"kind": "comment", "id": "c1", "author_name": "Steve", "comment_type": "comment",
		"body":       "expand this more",
		"anchor":     map[string]any{"line_start": 1, "line_end": 1, "quoted_text": "P"},
		"created_at": "2026-04-29T00:00:00Z",
	})
	postEv(t, map[string]any{
		"kind": "edit", "id": "e1", "comment_id": "c1", "author_name": "Ben",
		"body":       "the goal section needs explicit success criteria for 'validated'",
		"created_at": "2026-04-29T00:05:00Z",
	})
	// Mallory tries to edit too — must be ignored.
	postEv(t, map[string]any{
		"kind": "edit", "id": "e2", "comment_id": "c1", "author_name": "Mallory",
		"body":       "MALICIOUS",
		"created_at": "2026-04-29T00:06:00Z",
	})

	out := captureStdout(t, func() { _ = runShareComments(shareCommentsCmd, []string{s.ID}) })

	if !strings.Contains(out, "explicit success criteria") {
		t.Errorf("expected plan author's edited body in output; got:\n%s", out)
	}
	if strings.Contains(out, "expand this more") {
		t.Errorf("expected stale body to be replaced; got:\n%s", out)
	}
	// Author attribution unchanged: the line should still be tagged with Steve.
	if !strings.Contains(out, "Steve") {
		t.Errorf("comment.author_name should still be Steve in the output; got:\n%s", out)
	}
	if strings.Contains(out, "MALICIOUS") {
		t.Errorf("third-party edit must be ignored; got:\n%s", out)
	}
}

// TestRunShareCommentsJSONBundle verifies the shape of `--json` output:
// a single JSON object containing plan metadata + comments with action
// preserved + resolved_anchor populated against the on-disk plan file.
//
// This is the contract that `arc share comments --json` exposes to the
// brainstorm skill / LLM agents — locking it in here so changes that
// would break agent consumption fail the test.
func TestRunShareCommentsJSONBundle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	srv := startTestPasteServer(t)
	defer srv.Close()

	// A small but realistic plan with sections the SPA would slugify.
	planText := "# Test Plan\n\n## Goal\n\nValidate the shared review feature.\n\n## Approach\n\n- Selection-based annotation\n- Conventional labels\n"
	planFile := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(planFile, []byte(planText), 0o644); err != nil {
		t.Fatal(err)
	}
	shareCreateServer = srv.URL
	if err := runShareCreate(shareCreateCmd, []string{planFile}); err != nil {
		t.Fatal(err)
	}
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

	// One regular comment + one delete annotation. The delete tests that
	// `action` round-trips to the JSON output (was the bug — Go side used
	// to silently drop it).
	postEv(t, map[string]any{
		"kind":         "comment",
		"id":           "c1",
		"author_name":  "Steve",
		"comment_type": "issue",
		"body":         "Goal section should mention success criteria",
		"anchor": map[string]any{
			"line_start":   5,
			"line_end":     5,
			"quoted_text":  "Validate the shared review feature.",
			"heading_slug": "goal",
		},
		"created_at": "2026-04-29T00:00:00Z",
	})
	postEv(t, map[string]any{
		"kind":         "comment",
		"id":           "c2",
		"author_name":  "Mike",
		"comment_type": "comment",
		"action":       "delete",
		"body":         "",
		"anchor": map[string]any{
			"line_start":   9,
			"line_end":     9,
			"quoted_text":  "Conventional labels",
			"heading_slug": "approach",
		},
		"created_at": "2026-04-29T00:01:00Z",
	})

	// Run with --json
	shareCommentsJSON = true
	t.Cleanup(func() { shareCommentsJSON = false })
	out := captureStdout(t, func() { _ = runShareComments(shareCommentsCmd, []string{s.ID}) })

	var bundle struct {
		Plan struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			File        string `json:"file"`
			MarkdownB64 string `json:"markdown_b64"`
		} `json:"plan"`
		Comments []struct {
			Comment struct {
				ID         string `json:"id"`
				Action     string `json:"action"`
				AuthorName string `json:"author_name"`
				Body       string `json:"body"`
			} `json:"comment"`
			Status         string `json:"status"`
			ResolvedAnchor *struct {
				Status    string `json:"status"`
				LineStart int    `json:"line_start"`
				LineEnd   int    `json:"line_end"`
				Snippet   string `json:"snippet"`
			} `json:"resolved_anchor"`
		} `json:"comments"`
	}
	if err := json.Unmarshal([]byte(out), &bundle); err != nil {
		t.Fatalf("output is not valid JSON bundle: %v\noutput:\n%s", err, out)
	}

	// --- Plan section ---
	if bundle.Plan.ID != s.ID {
		t.Errorf("plan.id = %q, want %q", bundle.Plan.ID, s.ID)
	}
	// File-readable case: `file` is set, `markdown_b64` MUST be omitted.
	// The agent reads the file directly — no need to ship its bytes twice.
	if !strings.HasSuffix(bundle.Plan.File, "plan.md") {
		t.Errorf("plan.file should be absolute path ending in plan.md; got %q", bundle.Plan.File)
	}
	if !filepath.IsAbs(bundle.Plan.File) {
		t.Errorf("plan.file should be absolute; got %q", bundle.Plan.File)
	}
	if bundle.Plan.MarkdownB64 != "" {
		t.Errorf("plan.markdown_b64 must be empty when plan.file is set; got %d bytes", len(bundle.Plan.MarkdownB64))
	}

	// --- Comments section ---
	if len(bundle.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(bundle.Comments))
	}
	// Sorted by created_at, so c1 comes before c2.
	if bundle.Comments[0].Comment.ID != "c1" || bundle.Comments[1].Comment.ID != "c2" {
		t.Errorf("comments not in chronological order: got %s, %s",
			bundle.Comments[0].Comment.ID, bundle.Comments[1].Comment.ID)
	}
	if bundle.Comments[1].Comment.Action != "delete" {
		t.Errorf("action field dropped on delete comment; got %q, want \"delete\"",
			bundle.Comments[1].Comment.Action)
	}

	// --- Resolved anchor ---
	r0 := bundle.Comments[0].ResolvedAnchor
	if r0 == nil {
		t.Fatal("resolved_anchor missing on c1")
	}
	if r0.Status != "ok" {
		t.Errorf("c1 anchor status = %q, want ok (line numbers should match)", r0.Status)
	}
	if r0.Snippet == "" {
		t.Errorf("expected snippet for resolved anchor; got empty")
	}
	if !strings.Contains(r0.Snippet, "Validate the shared review") {
		t.Errorf("snippet should include the quoted text; got %q", r0.Snippet)
	}
}

// TestRunShareCommentsJSONBundle_NoLocalFile covers the "agent on a
// different machine" case: the share isn't in this machine's shares.json
// (or the recorded file is unreadable), so the bundle must include the
// markdown as base64 instead of a file path.
func TestRunShareCommentsJSONBundle_NoLocalFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	srv := startTestPasteServer(t)
	defer srv.Close()

	// Create a share, then DELETE the registry entry to simulate "this
	// machine doesn't know about this share." The encrypted blob still
	// has the plan content, so the CLI falls back to it.
	planText := "# Test Plan\n\n## Goal\n\nA quick \"quoted\" test with newlines.\n"
	planFile := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(planFile, []byte(planText), 0o644); err != nil {
		t.Fatal(err)
	}
	shareCreateServer = srv.URL
	if err := runShareCreate(shareCreateCmd, []string{planFile}); err != nil {
		t.Fatal(err)
	}
	f, _ := sharesconfig.Load()
	s := f.Shares[0]
	keyBytes := mustDecodeKey(t, s.KeyB64Url)

	// Wipe shares.json so the lookup fails — same effect as fetching a
	// share you didn't create on this machine.
	if err := sharesconfig.Remove(s.ID); err != nil {
		t.Fatal(err)
	}

	// Also need to pass the full URL since the bare ID won't resolve now.
	url := srv.URL + "/share/" + s.ID + "#k=" + s.KeyB64Url
	_ = keyBytes

	shareCommentsJSON = true
	t.Cleanup(func() { shareCommentsJSON = false })
	out := captureStdout(t, func() { _ = runShareComments(shareCommentsCmd, []string{url}) })

	var bundle struct {
		Plan struct {
			File        string `json:"file"`
			MarkdownB64 string `json:"markdown_b64"`
		} `json:"plan"`
	}
	if err := json.Unmarshal([]byte(out), &bundle); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	if bundle.Plan.File != "" {
		t.Errorf("plan.file should be empty when share is not registered; got %q", bundle.Plan.File)
	}
	if bundle.Plan.MarkdownB64 == "" {
		t.Fatal("plan.markdown_b64 must be set when plan.file is empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(bundle.Plan.MarkdownB64)
	if err != nil {
		t.Fatalf("markdown_b64 not valid base64: %v", err)
	}
	if string(decoded) != planText {
		t.Errorf("decoded markdown_b64 doesn't match original.\n  got:  %q\n  want: %q",
			string(decoded), planText)
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
