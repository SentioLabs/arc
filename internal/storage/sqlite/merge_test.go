package sqlite_test

import (
	"context"
	"testing"

	"github.com/sentiolabs/arc/internal/types"
)

func TestMergeProjects_Basic(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create target and source projects
	target := &types.Project{Name: "Target", Prefix: "tgt"}
	if err := store.CreateProject(ctx, target); err != nil {
		t.Fatalf("create target project: %v", err)
	}
	source := &types.Project{Name: "Source", Prefix: "src"}
	if err := store.CreateProject(ctx, source); err != nil {
		t.Fatalf("create source project: %v", err)
	}

	// Create issues in both projects
	setupTestIssue(t, store, target, "Target Issue 1")
	setupTestIssue(t, store, source, "Source Issue 1")
	setupTestIssue(t, store, source, "Source Issue 2")

	// Merge source into target
	result, err := store.MergeProjects(ctx, target.ID, []string{source.ID}, "test-actor")
	if err != nil {
		t.Fatalf("MergeProjects failed: %v", err)
	}

	// Assert result counts
	if result.IssuesMoved != 2 {
		t.Errorf("IssuesMoved = %d, want 2", result.IssuesMoved)
	}
	if len(result.SourcesDeleted) != 1 {
		t.Errorf("SourcesDeleted length = %d, want 1", len(result.SourcesDeleted))
	} else if result.SourcesDeleted[0] != source.ID {
		t.Errorf("SourcesDeleted[0] = %q, want %q", result.SourcesDeleted[0], source.ID)
	}
	if result.TargetProject.ID != target.ID {
		t.Errorf("TargetProject.ID = %q, want %q", result.TargetProject.ID, target.ID)
	}

	// Verify all issues now belong to target
	issues, err := store.ListIssues(ctx, types.IssueFilter{ProjectID: target.ID})
	if err != nil {
		t.Fatalf("ListIssues failed: %v", err)
	}
	if len(issues) != 3 {
		t.Errorf("target project issue count = %d, want 3", len(issues))
	}

	// Verify source project is deleted
	_, err = store.GetProject(ctx, source.ID)
	if err == nil {
		t.Error("source project should be deleted after merge")
	}
}

func TestMergeProjects_MultipleSources(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	target := &types.Project{Name: "Target", Prefix: "tgt"}
	if err := store.CreateProject(ctx, target); err != nil {
		t.Fatalf("create target: %v", err)
	}
	src1 := &types.Project{Name: "Source1", Prefix: "sr1"}
	if err := store.CreateProject(ctx, src1); err != nil {
		t.Fatalf("create source1: %v", err)
	}
	src2 := &types.Project{Name: "Source2", Prefix: "sr2"}
	if err := store.CreateProject(ctx, src2); err != nil {
		t.Fatalf("create source2: %v", err)
	}

	setupTestIssue(t, store, src1, "S1 Issue")
	setupTestIssue(t, store, src2, "S2 Issue 1")
	setupTestIssue(t, store, src2, "S2 Issue 2")

	result, err := store.MergeProjects(ctx, target.ID, []string{src1.ID, src2.ID}, "test-actor")
	if err != nil {
		t.Fatalf("MergeProjects failed: %v", err)
	}

	if result.IssuesMoved != 3 {
		t.Errorf("IssuesMoved = %d, want 3", result.IssuesMoved)
	}
	if len(result.SourcesDeleted) != 2 {
		t.Errorf("SourcesDeleted length = %d, want 2", len(result.SourcesDeleted))
	}

	// Verify both sources deleted
	_, err = store.GetProject(ctx, src1.ID)
	if err == nil {
		t.Error("source1 should be deleted")
	}
	_, err = store.GetProject(ctx, src2.ID)
	if err == nil {
		t.Error("source2 should be deleted")
	}

	// Verify all issues in target
	issues, err := store.ListIssues(ctx, types.IssueFilter{ProjectID: target.ID})
	if err != nil {
		t.Fatalf("ListIssues failed: %v", err)
	}
	if len(issues) != 3 {
		t.Errorf("target issue count = %d, want 3", len(issues))
	}
}

func TestMergeProjects_DependenciesPreserved(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	target := &types.Project{Name: "Target", Prefix: "tgt"}
	if err := store.CreateProject(ctx, target); err != nil {
		t.Fatalf("create target: %v", err)
	}
	source := &types.Project{Name: "Source", Prefix: "src"}
	if err := store.CreateProject(ctx, source); err != nil {
		t.Fatalf("create source: %v", err)
	}

	// Create issues with cross-project dependency
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
	_, err := store.MergeProjects(ctx, target.ID, []string{source.ID}, "test-actor")
	if err != nil {
		t.Fatalf("MergeProjects failed: %v", err)
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

func TestMergeProjects_TargetNotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	source := &types.Project{Name: "Source", Prefix: "src"}
	if err := store.CreateProject(ctx, source); err != nil {
		t.Fatalf("create source: %v", err)
	}

	_, err := store.MergeProjects(ctx, "nonexistent-id", []string{source.ID}, "test-actor")
	if err == nil {
		t.Error("MergeProjects should return error for nonexistent target")
	}
}

func TestMergeProjects_SourceNotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	target := &types.Project{Name: "Target", Prefix: "tgt"}
	if err := store.CreateProject(ctx, target); err != nil {
		t.Fatalf("create target: %v", err)
	}

	_, err := store.MergeProjects(ctx, target.ID, []string{"nonexistent-id"}, "test-actor")
	if err == nil {
		t.Error("MergeProjects should return error for nonexistent source")
	}
}

func TestMergeProjects_SourceEqualsTarget(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	proj := &types.Project{Name: "Same", Prefix: "same"}
	if err := store.CreateProject(ctx, proj); err != nil {
		t.Fatalf("create project: %v", err)
	}

	_, err := store.MergeProjects(ctx, proj.ID, []string{proj.ID}, "test-actor")
	if err == nil {
		t.Error("MergeProjects should return error when source equals target")
	}
}
