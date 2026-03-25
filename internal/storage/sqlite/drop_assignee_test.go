package sqlite_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/sentiolabs/arc/internal/types"
)

// TestIssueStructHasNoAssigneeField verifies the Assignee field was removed from types.Issue.
func TestIssueStructHasNoAssigneeField(t *testing.T) {
	issueType := reflect.TypeOf(types.Issue{})
	_, found := issueType.FieldByName("Assignee")
	if found {
		t.Error("types.Issue still has an Assignee field; it should have been removed")
	}
}

// TestIssueFilterHasNoAssigneeField verifies the Assignee field was removed from types.IssueFilter.
func TestIssueFilterHasNoAssigneeField(t *testing.T) {
	filterType := reflect.TypeOf(types.IssueFilter{})
	_, found := filterType.FieldByName("Assignee")
	if found {
		t.Error("types.IssueFilter still has an Assignee field; it should have been removed")
	}
}

// TestWorkFilterHasNoAssigneeFields verifies Assignee and Unassigned fields were removed from types.WorkFilter.
func TestWorkFilterHasNoAssigneeFields(t *testing.T) {
	filterType := reflect.TypeOf(types.WorkFilter{})
	_, found := filterType.FieldByName("Assignee")
	if found {
		t.Error("types.WorkFilter still has an Assignee field; it should have been removed")
	}
	_, found = filterType.FieldByName("Unassigned")
	if found {
		t.Error("types.WorkFilter still has an Unassigned field; it should have been removed")
	}
}

// TestCreateAndRetrieveIssueWithoutAssignee verifies issues work without assignee.
func TestCreateAndRetrieveIssueWithoutAssignee(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	issue := &types.Issue{
		ProjectID: proj.ID,
		Title:     "Test without assignee",
		Status:    types.StatusOpen,
		Priority:  2,
		IssueType: types.TypeTask,
	}
	if err := store.CreateIssue(ctx, issue, "test-actor"); err != nil {
		t.Fatalf("CreateIssue failed: %v", err)
	}

	got, err := store.GetIssue(ctx, issue.ID)
	if err != nil {
		t.Fatalf("GetIssue failed: %v", err)
	}

	if got.Title != "Test without assignee" {
		t.Errorf("got title %q, want %q", got.Title, "Test without assignee")
	}
}

// TestListIssuesWithoutAssigneeFilter verifies ListIssues works without assignee filtering.
func TestListIssuesWithoutAssigneeFilter(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	setupTestIssue(t, store, proj, "Issue A")
	setupTestIssue(t, store, proj, "Issue B")

	issues, err := store.ListIssues(ctx, types.IssueFilter{
		ProjectID: proj.ID,
	})
	if err != nil {
		t.Fatalf("ListIssues failed: %v", err)
	}
	if len(issues) != 2 {
		t.Errorf("got %d issues, want 2", len(issues))
	}
}

// TestGetReadyWorkWithoutAssigneeFilter verifies GetReadyWork works without assignee filtering.
func TestGetReadyWorkWithoutAssigneeFilter(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	setupTestIssue(t, store, proj, "Ready Issue")

	issues, err := store.GetReadyWork(ctx, types.WorkFilter{
		ProjectID: proj.ID,
	})
	if err != nil {
		t.Fatalf("GetReadyWork failed: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("got %d issues, want 1", len(issues))
	}
}
