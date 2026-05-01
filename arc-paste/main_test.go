package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sentiolabs/arc/internal/paste"
	pastesqlite "github.com/sentiolabs/arc/internal/paste/sqlite"
	_ "modernc.org/sqlite"
)

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := pastesqlite.Apply(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return newRouter(paste.NewHandlers(pastesqlite.New(db)))
}

func TestArcPasteCreate(t *testing.T) {
	e := newTestRouter(t)
	body, _ := json.Marshal(map[string]any{
		"plan_blob":  []byte{1, 2, 3},
		"plan_iv":    []byte{4, 5, 6},
		"schema_ver": 1,
	})
	req := httptest.NewRequest("POST", "/api/paste", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

// arc-paste only owns /api/paste/*. The arc SPA shell calls /api/v1/projects
// on boot; without an explicit guard those requests fall through to the SPA
// fallback and return HTML, which the browser fails to JSON.parse. Return a
// clean JSON 404 so the SPA gets a parseable error instead of a crash.
func TestArcPasteRejectsArcAPIProbes(t *testing.T) {
	e := newTestRouter(t)
	for _, path := range []string{"/api/v1/projects", "/api/v1/workspaces", "/api/v1/anything/nested"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: expected 404, got %d", path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Errorf("%s: expected JSON content-type, got %q", path, ct)
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Errorf("%s: body is not JSON: %v (%q)", path, err, rec.Body.String())
		}
	}
}

// arc-paste deployments don't serve the arc app shell — the only useful
// destination is /share/<id>. Returning 404 at the root mirrors the Caddy
// edge configuration so dev (localhost) and prod (arcpaste.company.com)
// behave identically.
func TestArcPasteRootIs404(t *testing.T) {
	e := newTestRouter(t)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 at /, got %d (body=%q)", rec.Code, rec.Body.String())
	}
}
