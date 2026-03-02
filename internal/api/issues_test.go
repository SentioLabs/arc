package api //nolint:testpackage // tests use internal helpers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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
	addTestDependency(t, e, wsID, child1ID, epicID, "parent-child")
	addTestDependency(t, e, wsID, child2ID, epicID, "parent-child")

	// List issues filtered by parent_id
	url := fmt.Sprintf("/api/v1/workspaces/%s/issues?parent_id=%s", wsID, epicID)
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
	url := fmt.Sprintf("/api/v1/workspaces/%s/issues", wsID)
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
