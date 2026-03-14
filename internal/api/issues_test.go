package api //nolint:testpackage // tests use internal helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/types"
)

func TestListIssuesParentIDFilter(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	// Create an epic (parent)
	epicID := createTestIssueWithType(t, e, wsID, "Auth Epic", "epic")

	// Create child issues with parent-child dependency to the epic
	child1ID := createTestIssue(t, e, wsID, "Child Task 1")
	child2ID := createTestIssue(t, e, wsID, "Child Task 2")

	// Create an unrelated issue (not a child of the epic)
	_ = createTestIssue(t, e, wsID, "Unrelated Task")

	// Add parent-child dependencies (child depends on epic)
	addTestDependency(t, e, wsID, testDep{child1ID, epicID, "parent-child"})
	addTestDependency(t, e, wsID, testDep{child2ID, epicID, "parent-child"})

	// List issues filtered by parent_id
	url := fmt.Sprintf("/api/v1/projects/%s/issues?parent_id=%s", wsID, epicID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("listIssues with parent_id returned %d: %s", rec.Code, rec.Body.String())
	}

	var resp paginatedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Marshal the data field back to JSON and then unmarshal as issues
	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("failed to marshal data: %v", err)
	}

	var issues []*types.Issue
	if err := json.Unmarshal(dataBytes, &issues); err != nil {
		t.Fatalf("failed to parse issues: %v", err)
	}

	// Should only return the two children, not the epic or unrelated issue
	if len(issues) != 2 {
		t.Errorf("expected 2 child issues, got %d", len(issues))
		for _, iss := range issues {
			t.Logf("  issue: %s %q", iss.ID, iss.Title)
		}
	}

	// Verify the returned issues are the children
	childIDs := map[string]bool{child1ID: false, child2ID: false}
	for _, iss := range issues {
		if _, ok := childIDs[iss.ID]; ok {
			childIDs[iss.ID] = true
		} else {
			t.Errorf("unexpected issue in results: %s %q", iss.ID, iss.Title)
		}
	}
	for id, found := range childIDs {
		if !found {
			t.Errorf("expected child issue %s not found in results", id)
		}
	}
}

func TestListIssuesWithoutParentIDReturnsAll(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	// Create several issues
	createTestIssue(t, e, wsID, "Issue 1")
	createTestIssue(t, e, wsID, "Issue 2")
	createTestIssue(t, e, wsID, "Issue 3")

	// List issues without parent_id filter - should return all
	url := fmt.Sprintf("/api/v1/projects/%s/issues", wsID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("listIssues returned %d: %s", rec.Code, rec.Body.String())
	}

	var resp paginatedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("failed to marshal data: %v", err)
	}

	var issues []*types.Issue
	if err := json.Unmarshal(dataBytes, &issues); err != nil {
		t.Fatalf("failed to parse issues: %v", err)
	}

	if len(issues) != 3 {
		t.Errorf("expected 3 issues (all), got %d", len(issues))
	}
}

// closeTestIssue sends a POST to close an issue with the given JSON body.
func closeTestIssue(t *testing.T, e *echo.Echo, wsID, issueID, body string) *httptest.ResponseRecorder {
	t.Helper()

	closeURL := fmt.Sprintf("/api/v1/projects/%s/issues/%s/close", wsID, issueID)
	req := httptest.NewRequest(http.MethodPost, closeURL, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	return rec
}

// updateTestIssueStatus updates the status of a test issue via the API.
func updateTestIssueStatus(t *testing.T, e *echo.Echo, wsID, issueID, status string) {
	t.Helper()

	body := fmt.Sprintf(`{"status": %q}`, status)
	url := fmt.Sprintf("/api/v1/projects/%s/issues/%s", wsID, issueID)
	req := httptest.NewRequest(http.MethodPut, url, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("failed to update issue status: %s", rec.Body.String())
	}
}

// fetchIssues is a test helper that GETs the given URL and returns parsed issues.
func fetchIssues(t *testing.T, e *echo.Echo, url string) []*types.Issue {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("listIssues returned %d: %s", rec.Code, rec.Body.String())
	}

	var resp paginatedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("failed to marshal data: %v", err)
	}
	var issues []*types.Issue
	if err := json.Unmarshal(dataBytes, &issues); err != nil {
		t.Fatalf("failed to parse issues: %v", err)
	}
	return issues
}

// assertIssueIDs verifies that issues contains exactly the expected IDs.
func assertIssueIDs(t *testing.T, issues []*types.Issue, expectedIDs ...string) {
	t.Helper()
	if len(issues) != len(expectedIDs) {
		t.Errorf("expected %d issues, got %d", len(expectedIDs), len(issues))
	}
	found := make(map[string]bool)
	for _, iss := range issues {
		found[iss.ID] = true
	}
	for _, id := range expectedIDs {
		if !found[id] {
			t.Errorf("expected issue %s not found in results", id)
		}
	}
}

func TestListIssuesMultiFilter(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	bugID := createTestIssueWithType(t, e, wsID, "Open Bug", "bug")
	featureID := createTestIssueWithType(t, e, wsID, "InProgress Feature", "feature")
	taskID := createTestIssueWithType(t, e, wsID, "Closed Task", "task")

	updateTestIssueStatus(t, e, wsID, featureID, "in_progress")
	closeTestIssue(t, e, wsID, taskID, `{}`)

	t.Run("multi_status_filter", func(t *testing.T) {
		url := fmt.Sprintf("/api/v1/projects/%s/issues?status=open&status=in_progress", wsID)
		issues := fetchIssues(t, e, url)
		assertIssueIDs(t, issues, bugID, featureID)
	})

	t.Run("multi_type_filter", func(t *testing.T) {
		url := fmt.Sprintf("/api/v1/projects/%s/issues?type=bug&type=feature", wsID)
		issues := fetchIssues(t, e, url)
		assertIssueIDs(t, issues, bugID, featureID)
	})
}

func TestCloseIssueCascadeField(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	// Create a parent epic and a child task
	epicID := createTestIssueWithType(t, e, wsID, "Epic", "epic")
	childID := createTestIssue(t, e, wsID, "Child Task")
	addTestDependency(t, e, wsID, testDep{childID, epicID, "parent-child"})

	// Close with cascade=true should close both parent and child
	rec := closeTestIssue(t, e, wsID, epicID, `{"cascade": true}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("closeIssue with cascade returned %d: %s", rec.Code, rec.Body.String())
	}

	// Verify the child was also closed
	childURL := fmt.Sprintf("/api/v1/projects/%s/issues/%s", wsID, childID)
	req := httptest.NewRequest(http.MethodGet, childURL, nil)
	childRec := httptest.NewRecorder()
	e.ServeHTTP(childRec, req)

	var child types.Issue
	if err := json.Unmarshal(childRec.Body.Bytes(), &child); err != nil {
		t.Fatalf("failed to parse child issue: %v", err)
	}

	if child.Status != types.StatusClosed {
		t.Errorf("child status = %q, want %q", child.Status, types.StatusClosed)
	}
}

func TestCloseIssueOpenChildren409(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	// Create a parent epic and a child task
	epicID := createTestIssueWithType(t, e, wsID, "Epic", "epic")
	childID := createTestIssue(t, e, wsID, "Child Task")
	addTestDependency(t, e, wsID, testDep{childID, epicID, "parent-child"})

	// Close without cascade (default false) should return 409
	rec := closeTestIssue(t, e, wsID, epicID, `{}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("closeIssue without cascade returned %d, want %d: %s",
			rec.Code, http.StatusConflict, rec.Body.String())
	}

	// Parse the response body
	var resp struct {
		Error        string        `json:"error"`
		Code         string        `json:"code"`
		OpenChildren []types.Issue `json:"open_children"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse 409 response: %v", err)
	}

	if resp.Code != "open_children" {
		t.Errorf("response code = %q, want %q", resp.Code, "open_children")
	}

	if len(resp.OpenChildren) != 1 {
		t.Fatalf("expected 1 open child, got %d", len(resp.OpenChildren))
	}

	if resp.OpenChildren[0].ID != childID {
		t.Errorf("open_children[0].ID = %q, want %q", resp.OpenChildren[0].ID, childID)
	}
}

func TestCloseIssueNoChildrenNoCascade(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	wsID := createTestWorkspace(t, e)

	// Create a simple issue with no children
	issueID := createTestIssue(t, e, wsID, "Simple Task")

	// Close without cascade should succeed (no children to worry about)
	rec := closeTestIssue(t, e, wsID, issueID, `{}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("closeIssue returned %d: %s", rec.Code, rec.Body.String())
	}

	var issue types.Issue
	if err := json.Unmarshal(rec.Body.Bytes(), &issue); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if issue.Status != types.StatusClosed {
		t.Errorf("status = %q, want %q", issue.Status, types.StatusClosed)
	}
}
