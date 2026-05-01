package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	cfgpkg "github.com/sentiolabs/arc/internal/config"
)

func newTestServerWithTempHome(t *testing.T) *Server {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Pre-seed a default config so DefaultPath() resolves cleanly.
	if err := os.MkdirAll(filepath.Join(home, ".arc"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := cfgpkg.Save(cfgpkg.DefaultPath(), cfgpkg.Default()); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	return &Server{echo: echo.New()}
}

func TestGetConfigReturnsDefaults(t *testing.T) {
	s := newTestServerWithTempHome(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	rec := httptest.NewRecorder()
	c := s.echo.NewContext(req, rec)
	if err := s.getConfig(c); err != nil {
		t.Fatalf("getConfig: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"cli", "server", "share", "updates", "meta"} {
		if got[key] == nil {
			t.Errorf("response missing key %q: %v", key, got)
		}
	}
	cli, ok := got["cli"].(map[string]any)
	if !ok {
		t.Fatalf("cli is not an object: %T", got["cli"])
	}
	if cli["server"] != "http://localhost:7432" {
		t.Errorf("cli.server = %q, want %q", cli["server"], "http://localhost:7432")
	}
	meta, ok := got["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta is not an object: %T", got["meta"])
	}
	requiresRestart, ok := meta["requires_restart"].([]any)
	if !ok || len(requiresRestart) == 0 {
		t.Errorf("meta.requires_restart is empty or missing: %v", meta["requires_restart"])
	}
}

func TestPutConfigPersistsAndRevalidates(t *testing.T) {
	s := newTestServerWithTempHome(t)
	in := cfgpkg.Default()
	in.Share.Author = "Grace"
	body, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/v1/config", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := s.echo.NewContext(req, rec)
	if err := s.putConfig(c); err != nil {
		t.Fatalf("putConfig: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body)
	}

	// Assert response body contains share.author and meta.
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	share, ok := got["share"].(map[string]any)
	if !ok {
		t.Fatalf("share is not an object: %T", got["share"])
	}
	if share["author"] != "Grace" {
		t.Errorf("response share.author = %q, want %q", share["author"], "Grace")
	}
	if got["meta"] == nil {
		t.Errorf("response missing meta field")
	}

	// Also verify disk state.
	reloaded, err := cfgpkg.Load(cfgpkg.DefaultPath())
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Share.Author != "Grace" {
		t.Errorf("disk share.author = %q", reloaded.Share.Author)
	}
}

func TestPutConfigRejectsInvalid(t *testing.T) {
	s := newTestServerWithTempHome(t)
	in := cfgpkg.Default()
	in.Server.Port = 0
	body, _ := json.Marshal(in)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/config", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := s.echo.NewContext(req, rec)
	if err := s.putConfig(c); err != nil {
		t.Fatalf("putConfig: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", rec.Code, rec.Body)
	}
	var resp configValidationErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := resp.Errors["server.port"]; !ok {
		t.Errorf("missing server.port in errors: %v", resp.Errors)
	}
}
