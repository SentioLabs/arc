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
	if got["cli"] == nil || got["meta"] == nil {
		t.Errorf("response missing fields: %v", got)
	}
}

func TestPutConfigPersistsAndRevalidates(t *testing.T) {
	s := newTestServerWithTempHome(t)
	in := cfgpkg.Default()
	in.Share.Author = "Grace"
	body, _ := json.Marshal(in)
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

	reloaded, err := cfgpkg.Load(cfgpkg.DefaultPath())
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Share.Author != "Grace" {
		t.Errorf("share.author = %q", reloaded.Share.Author)
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
