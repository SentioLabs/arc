package main

import (
	"bytes"
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

	"github.com/sentiolabs/arc/internal/api"
	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/paste"
	pastesqlite "github.com/sentiolabs/arc/internal/paste/sqlite"
	"github.com/sentiolabs/arc/internal/sharesconfig"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
)

func startTestPasteServer(t *testing.T) *httptest.Server {
	t.Helper()
	// Open an arc storage backed by a temp-file sqlite db so the full
	// migration set (including 017_shares.sql) is applied. The paste
	// subsystem migrations are run by sqlite.New itself, so the same db
	// connection serves both /api/paste and /api/v1/shares.
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("sqlite.New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	e := echo.New()
	paste.NewHandlers(pastesqlite.New(store.DB())).Register(e.Group("/api/paste"))

	// Mount the share keyring routes on /api/v1 against the same store so
	// sharesconfig.{Load,Add,Find,Remove} hits a real handler chain.
	apiSrv := api.New(api.Config{Store: store})
	apiSrv.RegisterShareRoutes(e.Group("/api/v1"))

	srv := httptest.NewServer(e)
	t.Cleanup(srv.Close)

	// Inject a client pointed at this test server so sharesconfig calls
	// from CLI command code reach our in-process handlers. Restore the
	// production factory on test exit so other tests in this package
	// (and the CLI's main init) still see the real getClient wiring.
	sharesconfig.SetClientFactory(func() (sharesconfig.Client, error) {
		return client.New(srv.URL), nil
	})
	t.Cleanup(func() {
		sharesconfig.SetClientFactory(func() (sharesconfig.Client, error) {
			return getClient()
		})
	})

	return srv
}

func TestShareCreateRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	srv := startTestPasteServer(t)
	defer srv.Close()

	plan := filepath.Join(t.TempDir(), "plan.md")
	_ = os.WriteFile(plan, []byte("# Hello\n\nBody."), 0o600)

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
	_ = os.WriteFile(plan, []byte("# P"), 0o600)
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

// TestRunShareCommentsHidesRetracted verifies that a retraction event from the
// comment's original author removes the comment from `arc share comments`
// default output, while a forged retraction (mismatched author_name) is
// silently dropped at replay time. The encrypted retraction event remains in
// the log for audit.
func TestRunShareCommentsHidesRetracted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	srv := startTestPasteServer(t)
	defer srv.Close()

	plan := filepath.Join(t.TempDir(), "p.md")
	_ = os.WriteFile(plan, []byte("# P"), 0o600)
	shareCreateServer = srv.URL
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

	// Two comments: Alice's (which she'll retract) and Bob's (which she can't).
	postEv(t, map[string]any{
		"kind": "comment", "id": "c1", "author_name": "Alice", "comment_type": "comment",
		"body":       "regret posting this",
		"anchor":     map[string]any{"line_start": 1, "line_end": 1, "quoted_text": "P"},
		"created_at": "2026-04-29T00:00:00Z",
	})
	postEv(t, map[string]any{
		"kind": "comment", "id": "c2", "author_name": "Bob", "comment_type": "comment",
		"body":       "stays visible",
		"anchor":     map[string]any{"line_start": 1, "line_end": 1, "quoted_text": "P"},
		"created_at": "2026-04-29T00:01:00Z",
	})

	// Alice retracts her own — must take effect.
	postEv(t, map[string]any{
		"kind": "retraction", "id": "x1", "comment_id": "c1", "author_name": "Alice",
		"created_at": "2026-04-29T00:02:00Z",
	})
	// Mallory tries to retract Bob's — must be silently dropped.
	postEv(t, map[string]any{
		"kind": "retraction", "id": "x2", "comment_id": "c2", "author_name": "Mallory",
		"created_at": "2026-04-29T00:03:00Z",
	})

	out := captureStdout(t, func() { _ = runShareComments(shareCommentsCmd, []string{s.ID}) })

	if strings.Contains(out, "regret posting this") {
		t.Errorf("retracted comment must be hidden; got:\n%s", out)
	}
	if !strings.Contains(out, "stays visible") {
		t.Errorf("forged retraction should not have removed Bob's comment; got:\n%s", out)
	}
	if !strings.Contains(out, "Bob") {
		t.Errorf("expected Bob's comment to still be present; got:\n%s", out)
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
	_ = os.WriteFile(plan, []byte("# P"), 0o600)
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
	_ = os.WriteFile(plan, []byte("# P"), 0o600)
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
	planText := "# Test Plan\n\n## Goal\n\nValidate the shared review feature.\n\n" +
		"## Approach\n\n- Selection-based annotation\n- Conventional labels\n"
	planFile := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(planFile, []byte(planText), 0o600); err != nil {
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
	if err := os.WriteFile(planFile, []byte(planText), 0o600); err != nil {
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
	// Variable URL is intentional — tests post to httptest.Server URLs.
	resp, err := http.Post(url, "application/json", bytes.NewReader(body)) //nolint:gosec // G107: test-controlled URL
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

	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// TestResolveAuthor locks in the resolution precedence:
//
//	flag > config file > env var > git config (lowest)
//
// We isolate from the user's real ~/.arc/cli-config.json by pointing the
// global `configPath` at a temp file, and from the user's git identity by
// running each subtest with $PATH cleared so `git` is unavailable (the
// helper falls back silently to "" on git failure).
func TestShareCreatePrintsBothURLs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	srv := startTestPasteServer(t)
	defer srv.Close()

	plan := filepath.Join(t.TempDir(), "plan.md")
	_ = os.WriteFile(plan, []byte("# Hello\n\nBody."), 0o600)

	// Capture stdout.
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = stdout }()

	shareCreateServer = srv.URL
	if err := runShareCreate(shareCreateCmd, []string{plan}); err != nil {
		t.Fatalf("runShareCreate: %v", err)
	}
	_ = w.Close()
	out, _ := io.ReadAll(r)
	output := string(out)

	if !strings.Contains(output, "Share URL") {
		t.Errorf("expected 'Share URL' label, got: %s", output)
	}
	if !strings.Contains(output, "Author URL") {
		t.Errorf("expected 'Author URL' label, got: %s", output)
	}
	if !strings.Contains(output, "send to reviewers") {
		t.Errorf("expected reviewer guidance, got: %s", output)
	}
	if !strings.Contains(output, "keep private") {
		t.Errorf("expected privacy guidance for author URL, got: %s", output)
	}
	if strings.Contains(output, "Edit token: ") {
		t.Errorf("raw 'Edit token: <hex>' line should not appear in output: %s", output)
	}
	if !strings.Contains(output, "Edit token saved to") {
		t.Errorf("expected pointer to shares.json, got: %s", output)
	}

	// Author URL must contain &t=.
	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, "/share/") && strings.Contains(line, "&t=") {
			return
		}
	}
	t.Errorf("expected an author URL line with &t=<token>, got: %s", output)
}

func TestResolveAuthor(t *testing.T) {
	// Save & restore globals touched by the helper. Env vars use t.Setenv
	// which auto-restores; configPath is package-global so we restore manually.
	origConfigPath := configPath
	t.Cleanup(func() { configPath = origConfigPath })

	// Strip git from PATH so the lowest tier resolves to "" deterministically.
	t.Setenv("PATH", "")

	writeConfig := func(t *testing.T, author string) {
		t.Helper()
		dir := t.TempDir()
		configPath = filepath.Join(dir, "cli-config.json")
		body := `{"server_url":"http://localhost:7432"`
		if author != "" {
			body += `,"share_author":"` + author + `"`
		}
		body += `}`
		if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}

	t.Run("flag wins over everything", func(t *testing.T) {
		writeConfig(t, "from-config")
		t.Setenv("ARC_SHARE_AUTHOR", "from-env")
		if got := resolveAuthor("from-flag"); got != "from-flag" {
			t.Errorf("flag should win; got %q", got)
		}
	})

	t.Run("config wins over env when no flag", func(t *testing.T) {
		writeConfig(t, "from-config")
		t.Setenv("ARC_SHARE_AUTHOR", "from-env")
		if got := resolveAuthor(""); got != "from-config" {
			t.Errorf("config should win over env; got %q", got)
		}
	})

	t.Run("env wins when config empty", func(t *testing.T) {
		writeConfig(t, "")
		t.Setenv("ARC_SHARE_AUTHOR", "from-env")
		if got := resolveAuthor(""); got != "from-env" {
			t.Errorf("env should win; got %q", got)
		}
	})

	t.Run("flag whitespace is trimmed and treated as empty", func(t *testing.T) {
		writeConfig(t, "from-config")
		t.Setenv("ARC_SHARE_AUTHOR", "")
		if got := resolveAuthor("   "); got != "from-config" {
			t.Errorf("whitespace flag should fall through; got %q", got)
		}
	})

	t.Run("everything empty resolves to empty string", func(t *testing.T) {
		writeConfig(t, "")
		t.Setenv("ARC_SHARE_AUTHOR", "")
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

	// Save & restore configPath manually; ARC_SHARE_SERVER uses t.Setenv
	// inside subtests for auto-restore.
	origConfigPath := configPath
	t.Cleanup(func() { configPath = origConfigPath })

	cases := []struct {
		name      string
		config    string // share_server in cli-config.json; "" omits the field
		env       string // ARC_SHARE_SERVER value; "" clears it
		share     bool
		local     bool
		flag      string
		wantURL   string
		wantKind  string
		wantNoEnv bool // true → don't t.Setenv at all (skip env priming)
	}{
		{
			name:   "flag wins over everything",
			config: "https://from-config.example", env: "https://from-env.example",
			share: true, flag: "https://from-flag.example",
			wantURL: "https://from-flag.example", wantKind: shareKindShared,
		},
		{
			// The override flag is intentionally global — it forces shared mode
			// regardless of which boolean flags are set, mirroring the previous
			// behavior. Locked in here so it doesn't silently drift.
			name:   "flag wins even without --share",
			config: "https://from-config.example",
			local:  true, flag: "https://from-flag.example", wantNoEnv: true,
			wantURL: "https://from-flag.example", wantKind: shareKindShared,
		},
		{
			name:   "config wins over env when no flag",
			config: "https://from-config.example", env: "https://from-env.example",
			share:   true,
			wantURL: "https://from-config.example", wantKind: shareKindShared,
		},
		{
			name: "env wins when config empty",
			env:  "https://from-env.example", share: true,
			wantURL: "https://from-env.example", wantKind: shareKindShared,
		},
		{
			name:    "falls back to built-in default",
			share:   true,
			wantURL: builtinDefault, wantKind: shareKindShared,
		},
		{
			name:   "flag whitespace is trimmed and treated as empty",
			config: "https://from-config.example",
			share:  true, flag: "   ",
			wantURL: "https://from-config.example", wantKind: shareKindShared,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeShareServerConfig(t, tc.config)
			if !tc.wantNoEnv {
				t.Setenv("ARC_SHARE_SERVER", tc.env)
			}
			got, kind := resolveServer(tc.local, tc.share, tc.flag)
			if got != tc.wantURL || kind != tc.wantKind {
				t.Errorf("resolveServer = (%q, %q), want (%q, %q)", got, kind, tc.wantURL, tc.wantKind)
			}
		})
	}
}

func TestShareShowAuthorURL(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	srv := startTestPasteServer(t)
	defer srv.Close()

	// Create a share so shares.json has an entry.
	plan := filepath.Join(t.TempDir(), "plan.md")
	_ = os.WriteFile(plan, []byte("# Hi"), 0o600)
	shareCreateServer = srv.URL
	if err := runShareCreate(shareCreateCmd, []string{plan}); err != nil {
		t.Fatal(err)
	}
	f, _ := sharesconfig.Load()
	id := f.Shares[0].ID
	editToken := f.Shares[0].EditToken
	keyB64 := f.Shares[0].KeyB64Url

	// Capture stdout for `arc share show <id> --author-url`.
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = stdout }()

	shareShowAuthorURL = true
	defer func() { shareShowAuthorURL = false }()
	if err := runShareShow(shareShowCmd, []string{id}); err != nil {
		t.Fatalf("runShareShow: %v", err)
	}
	_ = w.Close()
	out, _ := io.ReadAll(r)
	got := strings.TrimSpace(string(out))

	want := srv.URL + "/share/" + id + "#k=" + keyB64 + "&t=" + editToken
	if got != want {
		t.Errorf("author URL mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestShareShowAuthorURLMissingShare(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Bring up an empty in-process server so sharesconfig.Find returns
	// a clean ErrShareNotFound rather than trying to dial localhost:7432.
	_ = startTestPasteServer(t)

	shareShowAuthorURL = true
	defer func() { shareShowAuthorURL = false }()
	err := runShareShow(shareShowCmd, []string{"abc12345"})
	if err == nil {
		t.Fatal("expected error for missing share, got nil")
	}
	if !strings.Contains(err.Error(), "abc12345") {
		t.Errorf("error should reference share id, got: %v", err)
	}
}

// writeShareServerConfig points the global configPath at a temp cli-config.json
// containing the given share_server (omitted entirely when empty).
func writeShareServerConfig(t *testing.T, server string) {
	t.Helper()
	dir := t.TempDir()
	configPath = filepath.Join(dir, "cli-config.json")
	body := `{"server_url":"http://localhost:7432"`
	if server != "" {
		body += `,"share_server":"` + server + `"`
	}
	body += `}`
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
