package api //nolint:testpackage // tests use internal helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentiolabs/arc/internal/types"
)

// TestProjectRoutes verifies that the API uses /api/v1/projects/ routes
// instead of the old /api/v1/workspaces/ routes for project (container) management.
func TestProjectRoutes(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	// Create a project via the new /projects route
	body := `{"name": "Test Project", "prefix": "tp"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/projects returned %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var ws types.Project
	if err := json.Unmarshal(rec.Body.Bytes(), &ws); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// List projects
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/projects returned %d, want %d", rec.Code, http.StatusOK)
	}

	// Get project by ID
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+ws.ID, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/projects/:id returned %d, want %d", rec.Code, http.StatusOK)
	}

	// Get project stats
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+ws.ID+"/stats", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/projects/:id/stats returned %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestProjectScopedIssueRoutes verifies that issue routes use /api/v1/projects/:ws/issues
func TestProjectScopedIssueRoutes(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	// Create project via new route
	body := `{"name": "Issue Project", "prefix": "ip"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create project: %s", rec.Body.String())
	}

	var ws types.Project
	if err := json.Unmarshal(rec.Body.Bytes(), &ws); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Create issue via project-scoped route
	issueBody := `{"title": "Test Issue", "priority": 2}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+ws.ID+"/issues", bytes.NewBufferString(issueBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/projects/:ws/issues returned %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var issue types.Issue
	if err := json.Unmarshal(rec.Body.Bytes(), &issue); err != nil {
		t.Fatalf("failed to parse issue: %v", err)
	}

	// List issues
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+ws.ID+"/issues", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/projects/:ws/issues returned %d, want %d", rec.Code, http.StatusOK)
	}

	// Get ready work
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+ws.ID+"/ready", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/projects/:ws/ready returned %d, want %d", rec.Code, http.StatusOK)
	}

	// Get team-context
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/team-context", ws.ID), nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/projects/:ws/team-context returned %d, want %d", rec.Code, http.StatusOK)
	}
}
