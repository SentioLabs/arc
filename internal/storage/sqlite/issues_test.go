package sqlite_test

import (
	"context"
	"testing"

	"github.com/sentiolabs/arc/internal/types"
)

func TestListIssuesMultiFilterStatuses(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	// Create 3 issues with different statuses
	issueOpen := setupTestIssue(t, store, proj, "Open Issue")
	// issueOpen is already StatusOpen from setupTestIssue

	issueInProgress := setupTestIssue(t, store, proj, "In Progress Issue")
	err := store.UpdateIssue(ctx, issueInProgress.ID,
		map[string]any{"status": string(types.StatusInProgress)}, "test-actor")
	if err != nil {
		t.Fatalf("failed to update issue status: %v", err)
	}

	issueClosed := setupTestIssue(t, store, proj, "Closed Issue")
	if err := store.CloseIssue(ctx, issueClosed.ID, "done", false, "test-actor"); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	// Filter by statuses: open + in_progress
	issues, err := store.ListIssues(ctx, types.IssueFilter{
		ProjectID: proj.ID,
		Statuses:  []types.Status{types.StatusOpen, types.StatusInProgress},
	})
	if err != nil {
		t.Fatalf("ListIssues with Statuses failed: %v", err)
	}

	if len(issues) != 2 {
		t.Errorf("ListIssues with Statuses returned %d issues, want 2", len(issues))
	}

	// Verify the returned issues are the expected ones
	foundIDs := make(map[string]bool)
	for _, iss := range issues {
		foundIDs[iss.ID] = true
	}
	if !foundIDs[issueOpen.ID] {
		t.Errorf("expected open issue %s in results", issueOpen.ID)
	}
	if !foundIDs[issueInProgress.ID] {
		t.Errorf("expected in_progress issue %s in results", issueInProgress.ID)
	}
	if foundIDs[issueClosed.ID] {
		t.Errorf("did not expect closed issue %s in results", issueClosed.ID)
	}
}

func TestListIssuesMultiFilterPriorities(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	// Create issues with different priorities.
	// Note: priority 0 is overwritten to 2 by SetDefaults, so we use 1, 3, 4.
	issue1 := &types.Issue{
		ProjectID: proj.ID,
		Title:     "High Priority Issue",
		Status:    types.StatusOpen,
		Priority:  1,
		IssueType: types.TypeTask,
	}
	if err := store.CreateIssue(ctx, issue1, "test-actor"); err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	issue3 := &types.Issue{
		ProjectID: proj.ID,
		Title:     "Low Priority Issue",
		Status:    types.StatusOpen,
		Priority:  3,
		IssueType: types.TypeTask,
	}
	if err := store.CreateIssue(ctx, issue3, "test-actor"); err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	issue4 := &types.Issue{
		ProjectID: proj.ID,
		Title:     "Backlog Issue",
		Status:    types.StatusOpen,
		Priority:  4,
		IssueType: types.TypeTask,
	}
	if err := store.CreateIssue(ctx, issue4, "test-actor"); err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Filter by priorities: 1 and 3
	issues, err := store.ListIssues(ctx, types.IssueFilter{
		ProjectID:  proj.ID,
		Priorities: []int{1, 3},
	})
	if err != nil {
		t.Fatalf("ListIssues with Priorities failed: %v", err)
	}

	if len(issues) != 2 {
		t.Errorf("ListIssues with Priorities returned %d issues, want 2", len(issues))
	}

	foundIDs := make(map[string]bool)
	for _, iss := range issues {
		foundIDs[iss.ID] = true
	}
	if !foundIDs[issue1.ID] {
		t.Errorf("expected priority-1 issue %s in results", issue1.ID)
	}
	if !foundIDs[issue3.ID] {
		t.Errorf("expected priority-3 issue %s in results", issue3.ID)
	}
	if foundIDs[issue4.ID] {
		t.Errorf("did not expect priority-4 issue %s in results", issue4.ID)
	}
}

func TestListIssuesMultiFilterIssueTypes(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	// Create issues with different types
	issueBug := &types.Issue{
		ProjectID: proj.ID,
		Title:     "Bug Report",
		Status:    types.StatusOpen,
		Priority:  2,
		IssueType: types.TypeBug,
	}
	if err := store.CreateIssue(ctx, issueBug, "test-actor"); err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	issueFeature := &types.Issue{
		ProjectID: proj.ID,
		Title:     "Feature Request",
		Status:    types.StatusOpen,
		Priority:  2,
		IssueType: types.TypeFeature,
	}
	if err := store.CreateIssue(ctx, issueFeature, "test-actor"); err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	issueChore := &types.Issue{
		ProjectID: proj.ID,
		Title:     "Chore Item",
		Status:    types.StatusOpen,
		Priority:  2,
		IssueType: types.TypeChore,
	}
	if err := store.CreateIssue(ctx, issueChore, "test-actor"); err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Filter by issue types: bug + feature
	issues, err := store.ListIssues(ctx, types.IssueFilter{
		ProjectID:  proj.ID,
		IssueTypes: []types.IssueType{types.TypeBug, types.TypeFeature},
	})
	if err != nil {
		t.Fatalf("ListIssues with IssueTypes failed: %v", err)
	}

	if len(issues) != 2 {
		t.Errorf("ListIssues with IssueTypes returned %d issues, want 2", len(issues))
	}

	foundIDs := make(map[string]bool)
	for _, iss := range issues {
		foundIDs[iss.ID] = true
	}
	if !foundIDs[issueBug.ID] {
		t.Errorf("expected bug issue %s in results", issueBug.ID)
	}
	if !foundIDs[issueFeature.ID] {
		t.Errorf("expected feature issue %s in results", issueFeature.ID)
	}
	if foundIDs[issueChore.ID] {
		t.Errorf("did not expect chore issue %s in results", issueChore.ID)
	}
}

func TestListIssuesMultiFilterEmptySlicesReturnAll(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	// Create issues with different statuses
	_ = setupTestIssue(t, store, proj, "Issue 1")
	issue2 := setupTestIssue(t, store, proj, "Issue 2")
	err := store.UpdateIssue(ctx, issue2.ID,
		map[string]any{"status": string(types.StatusInProgress)}, "test-actor")
	if err != nil {
		t.Fatalf("failed to update issue: %v", err)
	}

	// Empty Statuses slice should return all issues (no status filter)
	issues, err := store.ListIssues(ctx, types.IssueFilter{
		ProjectID: proj.ID,
		Statuses:  []types.Status{},
	})
	if err != nil {
		t.Fatalf("ListIssues with empty Statuses failed: %v", err)
	}

	if len(issues) != 2 {
		t.Errorf("ListIssues with empty Statuses returned %d issues, want 2", len(issues))
	}
}

func TestListIssuesMultiFilterCombined(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	// Create: open bug p1, open feature p2, in_progress bug p1
	// Note: priority 0 is overwritten to 2 by SetDefaults, so we use 1 for "high".
	issue1 := &types.Issue{
		ProjectID: proj.ID,
		Title:     "Open Bug P1",
		Status:    types.StatusOpen,
		Priority:  1,
		IssueType: types.TypeBug,
	}
	if err := store.CreateIssue(ctx, issue1, "test-actor"); err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	issue2 := &types.Issue{
		ProjectID: proj.ID,
		Title:     "Open Feature P2",
		Status:    types.StatusOpen,
		Priority:  2,
		IssueType: types.TypeFeature,
	}
	if err := store.CreateIssue(ctx, issue2, "test-actor"); err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	issue3 := &types.Issue{
		ProjectID: proj.ID,
		Title:     "InProgress Bug P1",
		Status:    types.StatusOpen,
		Priority:  1,
		IssueType: types.TypeBug,
	}
	if err := store.CreateIssue(ctx, issue3, "test-actor"); err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	err := store.UpdateIssue(ctx, issue3.ID,
		map[string]any{"status": string(types.StatusInProgress)}, "test-actor")
	if err != nil {
		t.Fatalf("failed to update issue: %v", err)
	}

	// Filter: statuses=[open], issue_types=[bug], priorities=[1]
	// Should only return issue1 (open bug p1)
	issues, err := store.ListIssues(ctx, types.IssueFilter{
		ProjectID:  proj.ID,
		Statuses:   []types.Status{types.StatusOpen},
		IssueTypes: []types.IssueType{types.TypeBug},
		Priorities: []int{1},
	})
	if err != nil {
		t.Fatalf("ListIssues with combined filters failed: %v", err)
	}

	if len(issues) != 1 {
		t.Errorf("ListIssues with combined filters returned %d issues, want 1", len(issues))
	}
	if len(issues) > 0 && issues[0].ID != issue1.ID {
		t.Errorf("expected issue %s, got %s", issue1.ID, issues[0].ID)
	}
}
