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

// TestCreateIssueIgnoresAssignee verifies that the assignee field is not
// accepted in the create issue request body.
func TestCreateIssueIgnoresAssignee(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	pID := createTestProject(t, e)

	// Create issue with assignee in the body — should be ignored
	body := `{"title": "Test Task", "issue_type": "task", "priority": 2, "assignee": "alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pID+"/issues",
		bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Parse the response and verify no assignee field is populated
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if val, ok := raw["assignee"]; ok && val != "" {
		t.Errorf("expected no assignee in response, got %q", val)
	}
}

// TestUpdateIssueIgnoresAssignee verifies that the assignee field is not
// accepted in the update issue request body.
func TestUpdateIssueIgnoresAssignee(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	pID := createTestProject(t, e)
	issueID := createTestIssue(t, e, pID, "Test Task")

	// Update with assignee in body — should be ignored
	body := `{"assignee": "bob"}`
	url := fmt.Sprintf("/api/v1/projects/%s/issues/%s", pID, issueID)
	req := httptest.NewRequest(http.MethodPut, url, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Since assignee is removed, the only field in the body is unrecognized,
	// so "no updates provided" should be returned
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 (no updates), got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestListIssuesAssigneeParamIgnored verifies that the assignee query
// parameter is not used for filtering in listIssues.
func TestListIssuesAssigneeParamIgnored(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	pID := createTestProject(t, e)
	createTestIssue(t, e, pID, "Task 1")
	createTestIssue(t, e, pID, "Task 2")

	// Filter by assignee — should still return all issues (param ignored)
	url := fmt.Sprintf("/api/v1/projects/%s/issues?assignee=alice", pID)
	issues := fetchIssues(t, e, url)

	if len(issues) != 2 {
		t.Errorf("expected 2 issues (assignee param ignored), got %d", len(issues))
	}
}

// TestGetReadyWorkAssigneeParamsIgnored verifies that assignee and unassigned
// query parameters are not used for filtering in getReadyWork.
func TestGetReadyWorkAssigneeParamsIgnored(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	pID := createTestProject(t, e)
	createTestIssue(t, e, pID, "Ready Task 1")
	createTestIssue(t, e, pID, "Ready Task 2")

	// Query with assignee param — should return all ready work
	url := fmt.Sprintf("/api/v1/projects/%s/ready?assignee=alice", pID)
	issues := fetchReadyWork(t, e, url)
	if len(issues) != 2 {
		t.Errorf("expected 2 ready issues with assignee param, got %d", len(issues))
	}

	// Query with unassigned=true param — should also return all
	url = fmt.Sprintf("/api/v1/projects/%s/ready?unassigned=true", pID)
	issues = fetchReadyWork(t, e, url)
	if len(issues) != 2 {
		t.Errorf("expected 2 ready issues with unassigned param, got %d", len(issues))
	}
}

// TestTeamContextNoUnassignedField verifies that the team context response
// does not include an unassigned field.
func TestTeamContextNoUnassignedField(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	pID := createTestProject(t, e)

	// Create an epic and an unassigned child (no teammate label)
	epicID := createTestIssueWithType(t, e, pID, "Test Epic", "epic")
	childID := createTestIssue(t, e, pID, "Unlabeled Task")
	addTestDependency(t, e, pID, testDep{childID, epicID, "parent-child"})

	// Request team context with epic filter
	url := fmt.Sprintf("/api/v1/projects/%s/team-context?epic_id=%s", pID, epicID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Parse as raw JSON and verify no "unassigned" key
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if _, ok := raw["unassigned"]; ok {
		t.Error("team context response should not contain 'unassigned' field")
	}
}

// fetchReadyWork is a test helper that GETs the ready work endpoint and returns parsed issues.
func fetchReadyWork(t *testing.T, e *echo.Echo, url string) []map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getReadyWork returned %d: %s", rec.Code, rec.Body.String())
	}

	var issues []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &issues); err != nil {
		t.Fatalf("failed to parse ready work response: %v", err)
	}
	return issues
}
