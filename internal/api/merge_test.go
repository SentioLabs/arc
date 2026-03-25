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

func createNamedProject(t *testing.T, e *echo.Echo, name, prefix string) string {
	t.Helper()

	body := `{"name": "` + name + `", "prefix": "` + prefix + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create project %s: %s", name, rec.Body.String())
	}

	var p types.Project
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("failed to parse project response: %v", err)
	}
	return p.ID
}

func TestMergeProjects_Success(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	targetID := createNamedProject(t, e, "Target", "tgt")
	sourceID := createNamedProject(t, e, "Source", "src")

	// Create an issue in the source project
	issueBody := `{"title": "Source Issue"}`
	req := httptest.NewRequest(
		http.MethodPost, "/api/v1/projects/"+sourceID+"/issues",
		bytes.NewBufferString(issueBody),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create issue: %s", rec.Body.String())
	}

	// Merge source into target
	mergeBody := `{"target_id": "` + targetID + `", "source_ids": ["` + sourceID + `"]}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects/merge", bytes.NewBufferString(mergeBody))
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

	if result.TargetProject.ID != targetID {
		t.Errorf("target_project.id = %q, want %q", result.TargetProject.ID, targetID)
	}
	if result.IssuesMoved != 1 {
		t.Errorf("issues_moved = %d, want 1", result.IssuesMoved)
	}
	if len(result.SourcesDeleted) != 1 || result.SourcesDeleted[0] != sourceID {
		t.Errorf("sources_deleted = %v, want [%s]", result.SourcesDeleted, sourceID)
	}
}

func TestMergeProjects_MissingTargetID(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	mergeBody := `{"source_ids": ["some-id"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/merge", bytes.NewBufferString(mergeBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMergeProjects_MissingSourceIDs(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	mergeBody := `{"target_id": "some-id"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/merge", bytes.NewBufferString(mergeBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMergeProjects_InvalidTarget(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	sourceID := createNamedProject(t, e, "Source", "src")

	mergeBody := `{"target_id": "nonexistent", "source_ids": ["` + sourceID + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/merge", bytes.NewBufferString(mergeBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
