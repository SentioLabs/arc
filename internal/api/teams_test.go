package api //nolint:testpackage // tests use internal helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

// createTestIssueWithType creates an issue with a specific type and returns its ID.
func createTestIssueWithType(t *testing.T, e *echo.Echo, wsID, title, issueType string) string {
	t.Helper()

	body := fmt.Sprintf(`{"title": %q, "issue_type": %q, "priority": 2}`, title, issueType)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID+"/issues",
		bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create issue: %s", rec.Body.String())
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse issue response: %v", err)
	}
	return result.ID
}

// createTestLabel creates a global label.
func createTestLabel(t *testing.T, e *echo.Echo, name, color string) {
	t.Helper()

	body := fmt.Sprintf(`{"name": %q, "color": %q}`, name, color)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/labels", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create label %q: %s", name, rec.Body.String())
	}
}

// addTestLabelToIssue adds a label to an issue.
func addTestLabelToIssue(t *testing.T, e *echo.Echo, wsID, issueID, label string) {
	t.Helper()

	body := fmt.Sprintf(`{"label": %q}`, label)
	url := fmt.Sprintf("/api/v1/workspaces/%s/issues/%s/labels", wsID, issueID)
	req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent && rec.Code != http.StatusCreated {
		t.Fatalf("failed to add label %q to %s (status %d): %s", label, issueID, rec.Code, rec.Body.String())
	}
}

// addTestDependency creates a dependency between two issues.
func addTestDependency(t *testing.T, e *echo.Echo, wsID, issueID, dependsOnID, depType string) {
	t.Helper()

	body := fmt.Sprintf(`{"depends_on_id": %q, "type": %q}`, dependsOnID, depType)
	url := fmt.Sprintf("/api/v1/workspaces/%s/issues/%s/deps", wsID, issueID)
	req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to add dep %s->%s: %s", issueID, dependsOnID, rec.Body.String())
	}
}

func TestGetTeamContext(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	// Create teammate labels
	createTestLabel(t, e, "teammate:frontend", "#3b82f6")
	createTestLabel(t, e, "teammate:backend", "#22c55e")

	// Create epic
	epicID := createTestIssueWithType(t, e, wsID, "Auth System", "epic")

	// Create child issues with labels
	frontendID := createTestIssue(t, e, wsID, "Login form")
	addTestLabelToIssue(t, e, wsID, frontendID, "teammate:frontend")

	backendID := createTestIssue(t, e, wsID, "Auth API")
	addTestLabelToIssue(t, e, wsID, backendID, "teammate:backend")

	// Add parent-child deps (child depends on epic)
	addTestDependency(t, e, wsID, frontendID, epicID, "parent-child")
	addTestDependency(t, e, wsID, backendID, epicID, "parent-child")

	// Request team context with epic filter
	url := fmt.Sprintf("/api/v1/workspaces/%s/team-context?epic_id=%s", wsID, epicID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var ctx TeamContextResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &ctx); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if ctx.Workspace != wsID {
		t.Errorf("workspace = %q, want %q", ctx.Workspace, wsID)
	}

	if ctx.Epic == nil {
		t.Fatal("expected epic in response")
	}
	if ctx.Epic.ID != epicID {
		t.Errorf("epic.ID = %q, want %q", ctx.Epic.ID, epicID)
	}

	frontendRole, ok := ctx.Roles["frontend"]
	if !ok {
		t.Fatal("expected frontend role in response")
	}
	if len(frontendRole.Issues) != 1 {
		t.Errorf("frontend issues = %d, want 1", len(frontendRole.Issues))
	}

	backendRole, ok := ctx.Roles["backend"]
	if !ok {
		t.Fatal("expected backend role in response")
	}
	if len(backendRole.Issues) != 1 {
		t.Errorf("backend issues = %d, want 1", len(backendRole.Issues))
	}
}

func TestGetTeamContextNoEpic(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	// Create teammate labels
	createTestLabel(t, e, "teammate:frontend", "#3b82f6")

	// Create an issue with a teammate label
	issueID := createTestIssue(t, e, wsID, "Dashboard page")
	addTestLabelToIssue(t, e, wsID, issueID, "teammate:frontend")

	// Request team context without epic filter
	url := fmt.Sprintf("/api/v1/workspaces/%s/team-context", wsID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var ctx TeamContextResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &ctx); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if ctx.Epic != nil {
		t.Error("expected no epic when no epic_id param")
	}

	frontendRole, ok := ctx.Roles["frontend"]
	if !ok {
		t.Fatal("expected frontend role in response")
	}
	if len(frontendRole.Issues) != 1 {
		t.Errorf("frontend issues = %d, want 1", len(frontendRole.Issues))
	}
}

func TestGetTeamContextEmpty(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	// Request team context for workspace with no teammate-labeled issues
	url := fmt.Sprintf("/api/v1/workspaces/%s/team-context", wsID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var ctx TeamContextResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &ctx); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(ctx.Roles) != 0 {
		t.Errorf("expected empty roles, got %d", len(ctx.Roles))
	}
}
