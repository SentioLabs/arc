package api //nolint:testpackage // tests use internal helpers that access unexported fields

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/arc/internal/paste"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/web"
)

// testServerWithDB creates a test server with paste routes registered.
func testServerWithDB(t *testing.T) (*Server, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	server := New(ServerOptions{
		Address: ":0",
		Store:   store,
		DB:      store.DB(),
	})

	cleanup := func() {
		store.Close()
	}

	return server, cleanup
}

func TestPasteRoutesMounted(t *testing.T) {
	srv, cleanup := testServerWithDB(t)
	defer cleanup()

	body, _ := json.Marshal(paste.CreatePasteRequest{
		PlanBlob:  []byte{1, 2, 3},
		PlanIV:    []byte{4, 5, 6},
		SchemaVer: 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/paste", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp paste.CreatePasteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.ID == "" {
		t.Errorf("missing id in response: %+v", resp)
	}
	if resp.EditToken == "" {
		t.Errorf("missing edit_token in response: %+v", resp)
	}
}

func TestShareRouteFallsBackToSPA(t *testing.T) {
	if !web.Enabled {
		t.Skip("skipping SPA fallback test: webui not compiled (run with -tags webui)")
	}

	srv, cleanup := testServerWithDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/share/abc", nil)
	rec := httptest.NewRecorder()
	srv.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (SPA fallback), got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("<html")) {
		t.Errorf("expected HTML body, got: %s", rec.Body.String()[:200])
	}
}
