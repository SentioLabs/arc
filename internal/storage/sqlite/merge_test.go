package sqlite_test

import (
	"context"
	"testing"

	"github.com/sentiolabs/arc/internal/types"
)

func TestMergeWorkspaces_Basic(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create target and source workspaces
	target := &types.Workspace{Name: "Target", Prefix: "tgt"}
	if err := store.CreateWorkspace(ctx, target); err != nil {
		t.Fatalf("create target workspace: %v", err)
	}
	source := &types.Workspace{Name: "Source", Prefix: "src"}
	if err := store.CreateWorkspace(ctx, source); err != nil {
		t.Fatalf("create source workspace: %v", err)
	}

	// Create issues in both workspaces
	setupTestIssue(t, store, target, "Target Issue 1")
	setupTestIssue(t, store, source, "Source Issue 1")
	setupTestIssue(t, store, source, "Source Issue 2")

	// Merge source into target
	result, err := store.MergeWorkspaces(ctx, target.ID, []string{source.ID}, "test-actor")
	if err != nil {
		t.Fatalf("MergeWorkspaces failed: %v", err)
	}

	// Assert result counts
	if result.IssuesMoved != 2 {
		t.Errorf("IssuesMoved = %d, want 2", result.IssuesMoved)
	}
	if result.PlansMoved != 0 {
		t.Errorf("PlansMoved = %d, want 0", result.PlansMoved)
	}
	if len(result.SourcesDeleted) != 1 {
		t.Errorf("SourcesDeleted length = %d, want 1", len(result.SourcesDeleted))
	} else if result.SourcesDeleted[0] != source.ID {
		t.Errorf("SourcesDeleted[0] = %q, want %q", result.SourcesDeleted[0], source.ID)
	}
	if result.TargetWorkspace == nil {
		t.Fatal("TargetWorkspace should not be nil")
	}
	if result.TargetWorkspace.ID != target.ID {
		t.Errorf("TargetWorkspace.ID = %q, want %q", result.TargetWorkspace.ID, target.ID)
	}

	// Verify all issues now belong to target
	issues, err := store.ListIssues(ctx, types.IssueFilter{WorkspaceID: target.ID})
	if err != nil {
		t.Fatalf("ListIssues failed: %v", err)
	}
	if len(issues) != 3 {
		t.Errorf("target workspace issue count = %d, want 3", len(issues))
	}

	// Verify source workspace is deleted
	_, err = store.GetWorkspace(ctx, source.ID)
	if err == nil {
		t.Error("source workspace should be deleted after merge")
	}
}

func TestMergeWorkspaces_MultipleSources(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	target := &types.Workspace{Name: "Target", Prefix: "tgt"}
	if err := store.CreateWorkspace(ctx, target); err != nil {
		t.Fatalf("create target: %v", err)
	}
	src1 := &types.Workspace{Name: "Source1", Prefix: "sr1"}
	if err := store.CreateWorkspace(ctx, src1); err != nil {
		t.Fatalf("create source1: %v", err)
	}
	src2 := &types.Workspace{Name: "Source2", Prefix: "sr2"}
	if err := store.CreateWorkspace(ctx, src2); err != nil {
		t.Fatalf("create source2: %v", err)
	}

	setupTestIssue(t, store, src1, "S1 Issue")
	setupTestIssue(t, store, src2, "S2 Issue 1")
	setupTestIssue(t, store, src2, "S2 Issue 2")

	result, err := store.MergeWorkspaces(ctx, target.ID, []string{src1.ID, src2.ID}, "test-actor")
	if err != nil {
		t.Fatalf("MergeWorkspaces failed: %v", err)
	}

	if result.IssuesMoved != 3 {
		t.Errorf("IssuesMoved = %d, want 3", result.IssuesMoved)
	}
	if len(result.SourcesDeleted) != 2 {
		t.Errorf("SourcesDeleted length = %d, want 2", len(result.SourcesDeleted))
	}

	// Verify both sources deleted
	_, err = store.GetWorkspace(ctx, src1.ID)
	if err == nil {
		t.Error("source1 should be deleted")
	}
	_, err = store.GetWorkspace(ctx, src2.ID)
	if err == nil {
		t.Error("source2 should be deleted")
	}

	// Verify all issues in target
	issues, err := store.ListIssues(ctx, types.IssueFilter{WorkspaceID: target.ID})
	if err != nil {
		t.Fatalf("ListIssues failed: %v", err)
	}
	if len(issues) != 3 {
		t.Errorf("target issue count = %d, want 3", len(issues))
	}
}

func TestMergeWorkspaces_WithPlans(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	target := &types.Workspace{Name: "Target", Prefix: "tgt"}
	if err := store.CreateWorkspace(ctx, target); err != nil {
		t.Fatalf("create target: %v", err)
	}
	source := &types.Workspace{Name: "Source", Prefix: "src"}
	if err := store.CreateWorkspace(ctx, source); err != nil {
		t.Fatalf("create source: %v", err)
	}

	// Create plans in source workspace
	plan1 := &types.Plan{ID: "plan.001", WorkspaceID: source.ID, Title: "Plan 1", Content: "Content 1"}
	if err := store.CreatePlan(ctx, plan1); err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}
	plan2 := &types.Plan{ID: "plan.002", WorkspaceID: source.ID, Title: "Plan 2", Content: "Content 2"}
	if err := store.CreatePlan(ctx, plan2); err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	result, err := store.MergeWorkspaces(ctx, target.ID, []string{source.ID}, "test-actor")
	if err != nil {
		t.Fatalf("MergeWorkspaces failed: %v", err)
	}

	if result.PlansMoved != 2 {
		t.Errorf("PlansMoved = %d, want 2", result.PlansMoved)
	}

	// Verify plans are now in target workspace
	plans, err := store.ListPlans(ctx, target.ID)
	if err != nil {
		t.Fatalf("ListPlans failed: %v", err)
	}
	if len(plans) != 2 {
		t.Errorf("target plan count = %d, want 2", len(plans))
	}
}

func TestMergeWorkspaces_DependenciesPreserved(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	target := &types.Workspace{Name: "Target", Prefix: "tgt"}
	if err := store.CreateWorkspace(ctx, target); err != nil {
		t.Fatalf("create target: %v", err)
	}
	source := &types.Workspace{Name: "Source", Prefix: "src"}
	if err := store.CreateWorkspace(ctx, source); err != nil {
		t.Fatalf("create source: %v", err)
	}

	// Create issues with cross-workspace dependency
	targetIssue := setupTestIssue(t, store, target, "Target Issue")
	sourceIssue := setupTestIssue(t, store, source, "Source Issue")

	dep := &types.Dependency{
		IssueID:     sourceIssue.ID,
		DependsOnID: targetIssue.ID,
		Type:        types.DepBlocks,
	}
	if err := store.AddDependency(ctx, dep, "test-actor"); err != nil {
		t.Fatalf("AddDependency failed: %v", err)
	}

	// Merge source into target
	_, err := store.MergeWorkspaces(ctx, target.ID, []string{source.ID}, "test-actor")
	if err != nil {
		t.Fatalf("MergeWorkspaces failed: %v", err)
	}

	// Verify dependency still exists
	deps, err := store.GetDependencies(ctx, sourceIssue.ID)
	if err != nil {
		t.Fatalf("GetDependencies failed: %v", err)
	}
	if len(deps) != 1 {
		t.Errorf("dependency count = %d, want 1", len(deps))
	} else if deps[0].DependsOnID != targetIssue.ID {
		t.Errorf("dependency DependsOnID = %q, want %q", deps[0].DependsOnID, targetIssue.ID)
	}
}

func TestMergeWorkspaces_TargetNotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	source := &types.Workspace{Name: "Source", Prefix: "src"}
	if err := store.CreateWorkspace(ctx, source); err != nil {
		t.Fatalf("create source: %v", err)
	}

	_, err := store.MergeWorkspaces(ctx, "nonexistent-id", []string{source.ID}, "test-actor")
	if err == nil {
		t.Error("MergeWorkspaces should return error for nonexistent target")
	}
}

func TestMergeWorkspaces_SourceNotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	target := &types.Workspace{Name: "Target", Prefix: "tgt"}
	if err := store.CreateWorkspace(ctx, target); err != nil {
		t.Fatalf("create target: %v", err)
	}

	_, err := store.MergeWorkspaces(ctx, target.ID, []string{"nonexistent-id"}, "test-actor")
	if err == nil {
		t.Error("MergeWorkspaces should return error for nonexistent source")
	}
}

func TestMergeWorkspaces_SourceEqualsTarget(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	ws := &types.Workspace{Name: "Same", Prefix: "same"}
	if err := store.CreateWorkspace(ctx, ws); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	_, err := store.MergeWorkspaces(ctx, ws.ID, []string{ws.ID}, "test-actor")
	if err == nil {
		t.Error("MergeWorkspaces should return error when source equals target")
	}
}
