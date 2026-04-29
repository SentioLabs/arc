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
	id, server, key, err := resolveShareRef("https://share.arc.tools/share/abc12345#k=AAAA")
	if err != nil {
		t.Fatal(err)
	}
	if id != "abc12345" || server != "https://share.arc.tools" || len(key) == 0 {
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
