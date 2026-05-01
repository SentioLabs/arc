package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// arc-paste deployments only legitimately serve the share surface. Anything
// outside the allowlist (paths the arc SPA shell normally owns —
// /api/v1/projects on boot, /labels, /<projectId>/issues, /dashboard, /,
// etc.) must 404. Without this, the SPA wildcard registered by
// web.RegisterSPA would happily return the arc app shell HTML for those
// paths, which is both confusing and (for /api/v1/* boot probes) actively
// breaks the SPA because it can't JSON.parse HTML. The test mirrors the
// Caddyfile allowlist exactly so the in-binary guard and the edge stay
// in lockstep.
func TestArcPasteAllowlist(t *testing.T) {
	t.Run("rejected paths 404", func(t *testing.T) {
		e := newTestRouter(t)
		// A representative slice — not exhaustive. Includes the arc SPA boot
		// probes and a sampling of the routes the arc app shell normally owns,
		// plus shapes that look superficially like /share/<id> but aren't.
		rejected := []string{
			"/",
			"/labels",
			"/dashboard",
			"/teams",
			"/api/v1/projects",
			"/api/v1/workspaces",
			"/api/v1/anything/nested",
			"/share",          // missing id
			"/share/",         // empty id
			"/share/abc/sub",  // multi-segment after /share/
			"/_app",           // bare /_app (no trailing slash) is not the asset prefix
			"/api/pasteX",     // not /api/paste or /api/paste/
			"/robots.txt.bak", // not exactly /robots.txt
		}
		for _, path := range rejected {
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
			if rec.Code != http.StatusNotFound {
				t.Errorf("%s: expected 404, got %d (body=%q)", path, rec.Code, rec.Body.String())
			}
		}
	})

	t.Run("allowed paths reach handlers", func(t *testing.T) {
		e := newTestRouter(t)
		// /api/paste create — the existing happy path. Round-trips a real
		// CreatePasteRequest so we know the allowlist didn't accidentally
		// shadow the handler.
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
			t.Errorf("/api/paste: expected 201 from handler, got %d (body=%q)", rec.Code, rec.Body.String())
		}

		// Allowlisted GET paths must NOT 404. We don't assert 200 because the
		// embedded SPA may not be present in CLI-only test builds — what
		// matters is that the allowlist doesn't reject them.
		for _, path := range []string{"/share/abc123", "/_app/anything", "/robots.txt"} {
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
			if rec.Code == http.StatusNotFound && rec.Body.String() == "Not found" {
				t.Errorf("%s: rejected by allowlist (got 404 with allowlist body), want pass-through to handler", path)
			}
		}
	})
}

// Sanity-check the allowlist predicate directly so a regression in shape
// matching is obvious without spinning up the full router.
func TestArcPasteAllowedPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/api/paste", true},
		{"/api/paste/abc", true},
		{"/api/paste/abc/blobs", true},
		{"/_app/foo.js", true},
		{"/_app/immutable/chunk.abc.js", true},
		{"/robots.txt", true},
		{"/share/abc123", true},
		{"/", false},
		{"/labels", false},
		{"/share", false},
		{"/share/", false},
		{"/share/abc/sub", false},
		{"/_app", false},
		{"/api/pasteX", false},
		{"/api/v1/projects", false},
	}
	for _, tc := range cases {
		if got := arcPasteAllowedPath(tc.path); got != tc.want {
			t.Errorf("arcPasteAllowedPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
