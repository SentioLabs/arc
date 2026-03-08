package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/sentiolabs/arc/internal/types"
)

func TestCloseIssueNoCascadeNoChildren(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

	issue := setupTestIssue(t, store, ws, "Simple Issue")

	// Close with cascade=false should succeed when no children exist
	err := store.CloseIssue(ctx, issue.ID, "done", false, "test-actor")
	if err != nil {
		t.Fatalf("CloseIssue failed: %v", err)
	}

	// Verify issue is closed
	closed, err := store.GetIssue(ctx, issue.ID)
	if err != nil {
		t.Fatalf("GetIssue failed: %v", err)
	}
	if closed.Status != types.StatusClosed {
		t.Errorf("expected status closed, got %s", closed.Status)
	}
}

func TestCloseIssueNoCascadeWithOpenChildren(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

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

	// Close with cascade=false should return OpenChildrenError
	err := store.CloseIssue(ctx, parent.ID, "done", false, "test-actor")
	if err == nil {
		t.Fatal("expected error when closing parent with open children, got nil")
	}

	var openChildErr *types.OpenChildrenError
	if !errors.As(err, &openChildErr) {
		t.Fatalf("expected *types.OpenChildrenError, got %T: %v", err, err)
	}
	if openChildErr.IssueID != parent.ID {
		t.Errorf("OpenChildrenError.IssueID = %s, want %s", openChildErr.IssueID, parent.ID)
	}
	if len(openChildErr.Children) != 1 {
		t.Errorf("OpenChildrenError.Children has %d items, want 1", len(openChildErr.Children))
	}

	// Verify parent is still open
	p, _ := store.GetIssue(ctx, parent.ID)
	if p.Status != types.StatusOpen {
		t.Errorf("parent should still be open, got %s", p.Status)
	}
}

func TestCloseIssueCascadeClosesChildren(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

	parent := setupTestIssue(t, store, ws, "Parent")
	child1 := setupTestIssue(t, store, ws, "Child 1")
	child2 := setupTestIssue(t, store, ws, "Child 2")

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

	// Close with cascade=true should close parent and all children
	err := store.CloseIssue(ctx, parent.ID, "completed", true, "test-actor")
	if err != nil {
		t.Fatalf("CloseIssue cascade failed: %v", err)
	}

	// Verify all are closed
	for _, id := range []string{parent.ID, child1.ID, child2.ID} {
		issue, err := store.GetIssue(ctx, id)
		if err != nil {
			t.Fatalf("GetIssue(%s) failed: %v", id, err)
		}
		if issue.Status != types.StatusClosed {
			t.Errorf("issue %s status = %s, want closed", id, issue.Status)
		}
	}

	// Verify children have cascade reason
	for _, id := range []string{child1.ID, child2.ID} {
		issue, _ := store.GetIssue(ctx, id)
		expectedReason := fmt.Sprintf("completed (cascade closed by %s)", parent.ID)
		if issue.CloseReason != expectedReason {
			t.Errorf("issue %s close_reason = %q, want %q", id, issue.CloseReason, expectedReason)
		}
	}

	// Verify parent has original reason
	p, _ := store.GetIssue(ctx, parent.ID)
	if p.CloseReason != "completed" {
		t.Errorf("parent close_reason = %q, want %q", p.CloseReason, "completed")
	}
}

func TestCloseIssueCascadeDeepHierarchy(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

	// Create grandparent -> parent -> child hierarchy
	grandparent := setupTestIssue(t, store, ws, "Grandparent")
	parent := setupTestIssue(t, store, ws, "Parent")
	child := setupTestIssue(t, store, ws, "Child")

	// grandparent -> parent
	dep1 := &types.Dependency{
		IssueID:     parent.ID,
		DependsOnID: grandparent.ID,
		Type:        types.DepParentChild,
	}
	if err := store.AddDependency(ctx, dep1, "test-actor"); err != nil {
		t.Fatalf("AddDependency failed: %v", err)
	}

	// parent -> child
	dep2 := &types.Dependency{
		IssueID:     child.ID,
		DependsOnID: parent.ID,
		Type:        types.DepParentChild,
	}
	if err := store.AddDependency(ctx, dep2, "test-actor"); err != nil {
		t.Fatalf("AddDependency failed: %v", err)
	}

	// Cascade close from grandparent
	err := store.CloseIssue(ctx, grandparent.ID, "all done", true, "test-actor")
	if err != nil {
		t.Fatalf("CloseIssue cascade failed: %v", err)
	}

	// All three should be closed
	for _, id := range []string{grandparent.ID, parent.ID, child.ID} {
		issue, err := store.GetIssue(ctx, id)
		if err != nil {
			t.Fatalf("GetIssue(%s) failed: %v", id, err)
		}
		if issue.Status != types.StatusClosed {
			t.Errorf("issue %s status = %s, want closed", id, issue.Status)
		}
	}

	// Child and parent should have cascade reason referencing the grandparent
	expectedReason := fmt.Sprintf("all done (cascade closed by %s)", grandparent.ID)
	for _, id := range []string{parent.ID, child.ID} {
		issue, _ := store.GetIssue(ctx, id)
		if issue.CloseReason != expectedReason {
			t.Errorf("issue %s close_reason = %q, want %q", id, issue.CloseReason, expectedReason)
		}
	}
}

func TestCloseIssueCascadeSkipsAlreadyClosed(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

	parent := setupTestIssue(t, store, ws, "Parent")
	child1 := setupTestIssue(t, store, ws, "Open Child")
	child2 := setupTestIssue(t, store, ws, "Already Closed Child")

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

	// Close child2 first
	if err := store.CloseIssue(ctx, child2.ID, "pre-closed", false, "test-actor"); err != nil {
		t.Fatalf("pre-close failed: %v", err)
	}

	// Cascade close parent - should not error on already-closed child2
	err := store.CloseIssue(ctx, parent.ID, "done", true, "test-actor")
	if err != nil {
		t.Fatalf("CloseIssue cascade failed: %v", err)
	}

	// child2 should keep its original close reason
	c2, _ := store.GetIssue(ctx, child2.ID)
	if c2.CloseReason != "pre-closed" {
		t.Errorf("already-closed child close_reason = %q, want %q", c2.CloseReason, "pre-closed")
	}
}

func TestCloseIssueNoCascadeAllChildrenClosed(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

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

	// Close child first
	if err := store.CloseIssue(ctx, child.ID, "done", false, "test-actor"); err != nil {
		t.Fatalf("close child failed: %v", err)
	}

	// Close parent with cascade=false should succeed since all children are closed
	err := store.CloseIssue(ctx, parent.ID, "done", false, "test-actor")
	if err != nil {
		t.Fatalf("CloseIssue should succeed when all children are closed: %v", err)
	}
}
