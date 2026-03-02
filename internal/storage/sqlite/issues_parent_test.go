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
	ws := setupTestWorkspace(t, store)

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
		WorkspaceID: ws.ID,
		ParentID:    parent.ID,
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

func TestListIssuesByParentFilterEmpty(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

	// Create an issue with no children
	parent := setupTestIssue(t, store, ws, "Childless Epic")

	// Use ListIssues with ParentID filter - should return empty
	issues, err := store.ListIssues(ctx, types.IssueFilter{
		WorkspaceID: ws.ID,
		ParentID:    parent.ID,
	})
	if err != nil {
		t.Fatalf("ListIssues with ParentID failed: %v", err)
	}

	if len(issues) != 0 {
		t.Errorf("ListIssues with ParentID for childless parent returned %d issues, want 0", len(issues))
	}
}
