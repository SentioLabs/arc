package api //nolint:testpackage // tests use internal helpers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/types"
)

func createNamedWorkspace(t *testing.T, e *echo.Echo, name, prefix string) string {
	t.Helper()

	body := `{"name": "` + name + `", "prefix": "` + prefix + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create workspace %s: %s", name, rec.Body.String())
	}

	var ws types.Workspace
	if err := json.Unmarshal(rec.Body.Bytes(), &ws); err != nil {
		t.Fatalf("failed to parse workspace response: %v", err)
	}
	return ws.ID
}

func TestMergeWorkspaces_Success(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	targetID := createNamedWorkspace(t, e, "Target", "tgt")
	sourceID := createNamedWorkspace(t, e, "Source", "src")

	// Create an issue in the source workspace
	issueBody := `{"title": "Source Issue"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+sourceID+"/issues", bytes.NewBufferString(issueBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create issue: %s", rec.Body.String())
	}

	// Merge source into target
	mergeBody := `{"target_id": "` + targetID + `", "source_ids": ["` + sourceID + `"]}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/merge", bytes.NewBufferString(mergeBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("merge returned %d: %s", rec.Code, rec.Body.String())
	}

	var result types.MergeResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse merge response: %v", err)
	}

	if result.TargetWorkspace == nil {
		t.Fatal("target_workspace should not be nil")
	}
	if result.TargetWorkspace.ID != targetID {
		t.Errorf("target_workspace.id = %q, want %q", result.TargetWorkspace.ID, targetID)
	}
	if result.IssuesMoved != 1 {
		t.Errorf("issues_moved = %d, want 1", result.IssuesMoved)
	}
	if len(result.SourcesDeleted) != 1 || result.SourcesDeleted[0] != sourceID {
		t.Errorf("sources_deleted = %v, want [%s]", result.SourcesDeleted, sourceID)
	}
}

func TestMergeWorkspaces_MissingTargetID(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	mergeBody := `{"source_ids": ["some-id"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/merge", bytes.NewBufferString(mergeBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMergeWorkspaces_MissingSourceIDs(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	mergeBody := `{"target_id": "some-id"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/merge", bytes.NewBufferString(mergeBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMergeWorkspaces_InvalidTarget(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	sourceID := createNamedWorkspace(t, e, "Source", "src")

	mergeBody := `{"target_id": "nonexistent", "source_ids": ["` + sourceID + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/merge", bytes.NewBufferString(mergeBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
