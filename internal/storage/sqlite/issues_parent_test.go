package sqlite_test

import (
	"context"
	"testing"

	"github.com/sentiolabs/arc/internal/types"
)

func TestListIssuesByParentFilter(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestProject(t, store)

	// Create parent epic
	parent := setupTestIssue(t, store, ws, "Parent Epic")

	// Create child issues with parent-child dependencies
	child1 := setupTestIssue(t, store, ws, "Child Task 1")
	child2 := setupTestIssue(t, store, ws, "Child Task 2")

	// Create an unrelated issue (should not appear in results)
	_ = setupTestIssue(t, store, ws, "Unrelated Issue")

	// Add parent-child dependencies
	for _, child := range []*types.Issue{child1, child2} {
		dep := &types.Dependency{
			IssueID:     child.ID,
			DependsOnID: parent.ID,
			Type:        types.DepParentChild,
		}
		if err := store.AddDependency(ctx, dep, "test-actor"); err != nil {
			t.Fatalf("AddDependency failed: %v", err)
		}
	}

	// Use ListIssues with ParentID filter
	issues, err := store.ListIssues(ctx, types.IssueFilter{
		ProjectID: ws.ID,
		ParentID:  parent.ID,
	})
	if err != nil {
		t.Fatalf("ListIssues with ParentID failed: %v", err)
	}

	// Should return exactly the two child issues
	if len(issues) != 2 {
		t.Errorf("ListIssues with ParentID returned %d issues, want 2", len(issues))
	}

	// Verify the returned issues are the children
	childIDs := make(map[string]bool)
	for _, issue := range issues {
		childIDs[issue.ID] = true
	}
	if !childIDs[child1.ID] {
		t.Errorf("ListIssues with ParentID did not return child1 (ID: %s)", child1.ID)
	}
	if !childIDs[child2.ID] {
		t.Errorf("ListIssues with ParentID did not return child2 (ID: %s)", child2.ID)
	}
}

func TestGetOpenChildIssues(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestProject(t, store)

	// Create parent issue
	parent := setupTestIssue(t, store, ws, "Parent Epic")

	// Create child issues
	child1 := setupTestIssue(t, store, ws, "Open Child 1")
	child2 := setupTestIssue(t, store, ws, "Open Child 2")
	child3 := setupTestIssue(t, store, ws, "Closed Child")

	// Create an unrelated issue
	_ = setupTestIssue(t, store, ws, "Unrelated Issue")

	// Add parent-child dependencies
	for _, child := range []*types.Issue{child1, child2, child3} {
		dep := &types.Dependency{
			IssueID:     child.ID,
			DependsOnID: parent.ID,
			Type:        types.DepParentChild,
		}
		if err := store.AddDependency(ctx, dep, "test-actor"); err != nil {
			t.Fatalf("AddDependency failed: %v", err)
		}
	}

	// Close child3
	if err := store.CloseIssue(ctx, child3.ID, "done", false, "test-actor"); err != nil {
		t.Fatalf("CloseIssue failed: %v", err)
	}

	// Get open children - should return only child1 and child2
	openChildren, err := store.GetOpenChildIssues(ctx, parent.ID)
	if err != nil {
		t.Fatalf("GetOpenChildIssues failed: %v", err)
	}

	if len(openChildren) != 2 {
		t.Errorf("GetOpenChildIssues returned %d issues, want 2", len(openChildren))
	}

	childIDs := make(map[string]bool)
	for _, issue := range openChildren {
		childIDs[issue.ID] = true
	}
	if !childIDs[child1.ID] {
		t.Errorf("GetOpenChildIssues did not return child1 (ID: %s)", child1.ID)
	}
	if !childIDs[child2.ID] {
		t.Errorf("GetOpenChildIssues did not return child2 (ID: %s)", child2.ID)
	}
	if childIDs[child3.ID] {
		t.Errorf("GetOpenChildIssues returned closed child3 (ID: %s)", child3.ID)
	}
}

func TestGetOpenChildIssuesEmpty(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestProject(t, store)

	// Create an issue with no children
	parent := setupTestIssue(t, store, ws, "Childless Issue")

	openChildren, err := store.GetOpenChildIssues(ctx, parent.ID)
	if err != nil {
		t.Fatalf("GetOpenChildIssues failed: %v", err)
	}

	if len(openChildren) != 0 {
		t.Errorf("GetOpenChildIssues returned %d issues for childless parent, want 0", len(openChildren))
	}
}

func TestGetOpenChildIssuesAllClosed(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestProject(t, store)

	parent := setupTestIssue(t, store, ws, "Parent")
	child := setupTestIssue(t, store, ws, "Child")

	dep := &types.Dependency{
		IssueID:     child.ID,
		DependsOnID: parent.ID,
		Type:        types.DepParentChild,
	}
	if err := store.AddDependency(ctx, dep, "test-actor"); err != nil {
		t.Fatalf("AddDependency failed: %v", err)
	}

	// Close the child
	if err := store.CloseIssue(ctx, child.ID, "done", false, "test-actor"); err != nil {
		t.Fatalf("CloseIssue failed: %v", err)
	}

	// All children are closed, so result should be empty
	openChildren, err := store.GetOpenChildIssues(ctx, parent.ID)
	if err != nil {
		t.Fatalf("GetOpenChildIssues failed: %v", err)
	}

	if len(openChildren) != 0 {
		t.Errorf("GetOpenChildIssues returned %d issues when all children are closed, want 0", len(openChildren))
	}
}

func TestGetOpenChildIssuesIgnoresNonParentChildDeps(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestProject(t, store)

	issue1 := setupTestIssue(t, store, ws, "Issue 1")
	issue2 := setupTestIssue(t, store, ws, "Issue 2")

	// Add a "blocks" dependency (not parent-child)
	dep := &types.Dependency{
		IssueID:     issue2.ID,
		DependsOnID: issue1.ID,
		Type:        types.DepBlocks,
	}
	if err := store.AddDependency(ctx, dep, "test-actor"); err != nil {
		t.Fatalf("AddDependency failed: %v", err)
	}

	// Should return empty since there are no parent-child deps
	openChildren, err := store.GetOpenChildIssues(ctx, issue1.ID)
	if err != nil {
		t.Fatalf("GetOpenChildIssues failed: %v", err)
	}

	if len(openChildren) != 0 {
		t.Errorf("GetOpenChildIssues returned %d issues for blocks dependency, want 0", len(openChildren))
	}
}

func TestListIssuesByParentAndStatusFilter(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestProject(t, store)

	// Create parent and children
	parent := setupTestIssue(t, store, ws, "Parent Epic")
	child1 := setupTestIssue(t, store, ws, "Open Child")
	child2 := setupTestIssue(t, store, ws, "Closed Child")

	for _, child := range []*types.Issue{child1, child2} {
		dep := &types.Dependency{
			IssueID:     child.ID,
			DependsOnID: parent.ID,
			Type:        types.DepParentChild,
		}
		if err := store.AddDependency(ctx, dep, "test-actor"); err != nil {
			t.Fatalf("AddDependency failed: %v", err)
		}
	}

	// Close child2
	if err := store.CloseIssue(ctx, child2.ID, "done", false, "test-actor"); err != nil {
		t.Fatalf("CloseIssue failed: %v", err)
	}

	// Filter by parent + status=open should only return child1
	issues, err := store.ListIssues(ctx, types.IssueFilter{
		ProjectID: ws.ID,
		ParentID:  parent.ID,
		Statuses:  []types.Status{types.StatusOpen},
	})
	if err != nil {
		t.Fatalf("ListIssues with ParentID+Status failed: %v", err)
	}

	if len(issues) != 1 {
		t.Errorf("ListIssues with ParentID+Status returned %d issues, want 1", len(issues))
	}
	if len(issues) > 0 && issues[0].ID != child1.ID {
		t.Errorf("expected child1 (%s), got %s", child1.ID, issues[0].ID)
	}
}

func TestListIssuesByParentFilterEmpty(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestProject(t, store)

	// Create an issue with no children
	parent := setupTestIssue(t, store, ws, "Childless Epic")

	// Use ListIssues with ParentID filter - should return empty
	issues, err := store.ListIssues(ctx, types.IssueFilter{
		ProjectID: ws.ID,
		ParentID:  parent.ID,
	})
	if err != nil {
		t.Fatalf("ListIssues with ParentID failed: %v", err)
	}

	if len(issues) != 0 {
		t.Errorf("ListIssues with ParentID for childless parent returned %d issues, want 0", len(issues))
	}
}
