package api //nolint:testpackage // tests use internal helpers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
)

// testServer creates a test server with a temporary SQLite database.
func testServer(t *testing.T) (*Server, func()) {
	t.Helper()

	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	server := New(NewServerConfig{
		Address: ":0",
		Store:   store,
	})

	cleanup := func() {
		store.Close()
	}

	return server, cleanup
}

// createTestProject creates a project for testing and returns its ID.
func createTestProject(t *testing.T, e *echo.Echo) string {
	t.Helper()

	body := `{"name": "Test Workspace", "prefix": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create project: %s", rec.Body.String())
	}

	var ws types.Workspace
	if err := json.Unmarshal(rec.Body.Bytes(), &ws); err != nil {
		t.Fatalf("failed to parse project response: %v", err)
	}

	return ws.ID
}

// createTestIssue creates an issue for testing and returns its ID.
func createTestIssue(t *testing.T, e *echo.Echo, pID, title string) string {
	t.Helper()

	body := `{"title": "` + title + `", "type": "task", "priority": 2}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pID+"/issues", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create issue: %s", rec.Body.String())
	}

	var issue types.Issue
	if err := json.Unmarshal(rec.Body.Bytes(), &issue); err != nil {
		t.Fatalf("failed to parse issue response: %v", err)
	}

	return issue.ID
}
