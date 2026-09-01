package sqlite_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
)

// setupRoadmapIssue creates an open issue of the given type at the default priority.
func setupRoadmapIssue(t *testing.T, store *sqlite.Store, proj *types.Project,
	title string, issueType types.IssueType,
) *types.Issue {
	t.Helper()

	issue := &types.Issue{
		ProjectID: proj.ID,
		Title:     title,
		Status:    types.StatusOpen,
		IssueType: issueType,
	}
	if err := store.CreateIssue(context.Background(), issue, "test-actor"); err != nil {
		t.Fatalf("CreateIssue(%s) failed: %v", title, err)
	}
	return issue
}

// setRoadmapPriority sets an issue's priority after creation, which is the only
// way to reach priority 0 because SetDefaults rewrites it to 2.
func setRoadmapPriority(t *testing.T, store *sqlite.Store, issue *types.Issue, priority int) {
	t.Helper()
	err := store.UpdateIssue(context.Background(), issue.ID,
		map[string]any{"priority": priority}, "test-actor")
	if err != nil {
		t.Fatalf("UpdateIssue(%s, priority=%d) failed: %v", issue.ID, priority, err)
	}
	issue.Priority = priority
}

// addRoadmapDep links two issues with the given dependency type.
func addRoadmapDep(t *testing.T, store *sqlite.Store, issueID, dependsOnID string, depType types.DependencyType) {
	t.Helper()
	dep := &types.Dependency{
		IssueID:     issueID,
		DependsOnID: dependsOnID,
		Type:        depType,
	}
	if err := store.AddDependency(context.Background(), dep, "test-actor"); err != nil {
		t.Fatalf("AddDependency(%s -> %s, %s) failed: %v", issueID, dependsOnID, depType, err)
	}
}

// setRoadmapStatus updates an issue's status.
func setRoadmapStatus(t *testing.T, store *sqlite.Store, issueID string, status types.Status) {
	t.Helper()
	err := store.UpdateIssue(context.Background(), issueID,
		map[string]any{"status": string(status)}, "test-actor")
	if err != nil {
		t.Fatalf("UpdateIssue(%s, status=%s) failed: %v", issueID, status, err)
	}
}

// readyIDs maps issue IDs present in a ready-work result.
func readyIDs(issues []*types.ReadyIssue) map[string]*types.ReadyIssue {
	byID := make(map[string]*types.ReadyIssue, len(issues))
	for _, issue := range issues {
		byID[issue.ID] = issue
	}
	return byID
}

func TestGetReadyWorkRoadmapGatedByBlockedAncestor(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	release := setupRoadmapIssue(t, store, proj, "v1", types.TypeRelease)
	m1 := setupRoadmapIssue(t, store, proj, "M1", types.TypeMilestone)
	m2 := setupRoadmapIssue(t, store, proj, "M2", types.TypeMilestone)
	addRoadmapDep(t, store, m1.ID, release.ID, types.DepParentChild)
	addRoadmapDep(t, store, m2.ID, release.ID, types.DepParentChild)
	addRoadmapDep(t, store, m2.ID, m1.ID, types.DepBlocks)

	m1Task := setupRoadmapIssue(t, store, proj, "M1 Task", types.TypeTask)
	addRoadmapDep(t, store, m1Task.ID, m1.ID, types.DepParentChild)

	m2Task := setupRoadmapIssue(t, store, proj, "M2 Task", types.TypeTask)
	addRoadmapDep(t, store, m2Task.ID, m2.ID, types.DepParentChild)

	issues, err := store.GetReadyWork(ctx, types.WorkFilter{ProjectID: proj.ID})
	if err != nil {
		t.Fatalf("GetReadyWork failed: %v", err)
	}
	byID := readyIDs(issues)
	if _, ok := byID[m2Task.ID]; ok {
		t.Errorf("task under blocked milestone M2 should be gated out of ready work")
	}
	if _, ok := byID[m1Task.ID]; !ok {
		t.Errorf("task under ungated milestone M1 should be ready")
	}

	// Close M1's subtree, then M1 itself. M2 is no longer blocked.
	if err := store.CloseIssue(ctx, m1Task.ID, "done", false, "test-actor"); err != nil {
		t.Fatalf("CloseIssue(m1Task) failed: %v", err)
	}
	if err := store.CloseIssue(ctx, m1.ID, "done", false, "test-actor"); err != nil {
		t.Fatalf("CloseIssue(m1) failed: %v", err)
	}

	issues, err = store.GetReadyWork(ctx, types.WorkFilter{ProjectID: proj.ID})
	if err != nil {
		t.Fatalf("GetReadyWork after close failed: %v", err)
	}
	if _, ok := readyIDs(issues)[m2Task.ID]; !ok {
		t.Errorf("task under M2 should be ready once M1 is closed")
	}
}

func TestGetReadyWorkRoadmapGatedByDeferredAncestor(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	milestone := setupRoadmapIssue(t, store, proj, "Deferred Milestone", types.TypeMilestone)
	task := setupRoadmapIssue(t, store, proj, "Child Task", types.TypeTask)
	addRoadmapDep(t, store, task.ID, milestone.ID, types.DepParentChild)
	setRoadmapStatus(t, store, milestone.ID, types.StatusDeferred)

	issues, err := store.GetReadyWork(ctx, types.WorkFilter{ProjectID: proj.ID})
	if err != nil {
		t.Fatalf("GetReadyWork failed: %v", err)
	}
	if _, ok := readyIDs(issues)[task.ID]; ok {
		t.Errorf("task under deferred milestone should be gated out of ready work")
	}

	setRoadmapStatus(t, store, milestone.ID, types.StatusOpen)

	issues, err = store.GetReadyWork(ctx, types.WorkFilter{ProjectID: proj.ID})
	if err != nil {
		t.Fatalf("GetReadyWork after reopen failed: %v", err)
	}
	if _, ok := readyIDs(issues)[task.ID]; !ok {
		t.Errorf("task should be ready once its milestone is open")
	}
}

func TestGetReadyWorkRoadmapEffectivePriority(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	release := setupRoadmapIssue(t, store, proj, "v1", types.TypeRelease)
	setRoadmapPriority(t, store, release, 0)
	child := setupRoadmapIssue(t, store, proj, "Child of P0 release", types.TypeTask)
	addRoadmapDep(t, store, child.ID, release.ID, types.DepParentChild)
	standalone := setupRoadmapIssue(t, store, proj, "Standalone P1", types.TypeTask)
	setRoadmapPriority(t, store, standalone, 1)

	issues, err := store.GetReadyWork(ctx, types.WorkFilter{
		ProjectID:  proj.ID,
		SortPolicy: types.SortPolicyPriority,
	})
	if err != nil {
		t.Fatalf("GetReadyWork failed: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("GetReadyWork returned %d issues, want 2", len(issues))
	}
	if issues[0].ID != child.ID {
		t.Errorf("first ready issue = %s, want child %s", issues[0].ID, child.ID)
	}
	if issues[0].EffectivePriority != 0 {
		t.Errorf("child EffectivePriority = %d, want 0", issues[0].EffectivePriority)
	}
	if issues[0].Priority != 2 {
		t.Errorf("child Priority = %d, want 2 (own priority is unchanged)", issues[0].Priority)
	}
	if issues[1].ID != standalone.ID {
		t.Fatalf("second ready issue = %s, want standalone %s", issues[1].ID, standalone.ID)
	}
	if issues[1].EffectivePriority != 1 {
		t.Errorf("standalone EffectivePriority = %d, want 1", issues[1].EffectivePriority)
	}
	if len(issues[0].Path) != 1 || issues[0].Path[0] != "v1" {
		t.Errorf("child Path = %v, want [v1]", issues[0].Path)
	}
	if len(issues[1].Path) != 0 {
		t.Errorf("standalone Path = %v, want empty", issues[1].Path)
	}
}

func TestGetReadyWorkRoadmapEffectivePriorityHybridOlderBand(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	// The standalone is created first so that created_at ordering, the last
	// sort key, opposes the effective-priority ordering under test.
	standalone := setupRoadmapIssue(t, store, proj, "Standalone P1", types.TypeTask)
	setRoadmapPriority(t, store, standalone, 1)
	release := setupRoadmapIssue(t, store, proj, "v1", types.TypeRelease)
	setRoadmapPriority(t, store, release, 0)
	child := setupRoadmapIssue(t, store, proj, "Child of P0 release", types.TypeTask)
	addRoadmapDep(t, store, child.ID, release.ID, types.DepParentChild)

	// Push every issue into the hybrid policy's older band.
	if _, err := store.DB().ExecContext(ctx,
		`UPDATE issues SET updated_at = datetime('now','-3 days')`); err != nil {
		t.Fatalf("aging issues failed: %v", err)
	}
	var fresh int
	err := store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM issues WHERE updated_at >= datetime('now', '-48 hours')`).Scan(&fresh)
	if err != nil {
		t.Fatalf("counting fresh issues failed: %v", err)
	}
	if fresh != 0 {
		t.Fatalf("%d issues are still in the fresh band, want 0", fresh)
	}

	issues, err := store.GetReadyWork(ctx, types.WorkFilter{
		ProjectID:  proj.ID,
		SortPolicy: types.SortPolicyHybrid,
	})
	if err != nil {
		t.Fatalf("GetReadyWork failed: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("GetReadyWork returned %d issues, want 2", len(issues))
	}
	if issues[0].ID != child.ID {
		t.Errorf("first ready issue = %s, want child %s (effective priority 0)", issues[0].ID, child.ID)
	}
	if issues[0].EffectivePriority != 0 {
		t.Errorf("child EffectivePriority = %d, want 0", issues[0].EffectivePriority)
	}
	if issues[1].ID != standalone.ID {
		t.Errorf("second ready issue = %s, want standalone %s", issues[1].ID, standalone.ID)
	}
}

func TestGetReadyWorkRoadmapExcludesContainers(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	epic := setupRoadmapIssue(t, store, proj, "Epic", types.TypeEpic)
	release := setupRoadmapIssue(t, store, proj, "Release", types.TypeRelease)
	milestone := setupRoadmapIssue(t, store, proj, "Milestone", types.TypeMilestone)
	task := setupRoadmapIssue(t, store, proj, "Task", types.TypeTask)

	issues, err := store.GetReadyWork(ctx, types.WorkFilter{ProjectID: proj.ID})
	if err != nil {
		t.Fatalf("GetReadyWork failed: %v", err)
	}
	byID := readyIDs(issues)
	for _, container := range []*types.Issue{epic, release, milestone} {
		if _, ok := byID[container.ID]; ok {
			t.Errorf("container %s (%s) should not appear in ready work", container.Title, container.IssueType)
		}
	}
	if _, ok := byID[task.ID]; !ok {
		t.Errorf("non-container task should appear in ready work")
	}
	if len(issues) != 1 {
		t.Errorf("GetReadyWork returned %d issues, want 1", len(issues))
	}
}

func TestGetReadyWorkRoadmapUnderScoping(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	release := setupRoadmapIssue(t, store, proj, "v1", types.TypeRelease)
	milestone := setupRoadmapIssue(t, store, proj, "M1", types.TypeMilestone)
	addRoadmapDep(t, store, milestone.ID, release.ID, types.DepParentChild)
	descendant := setupRoadmapIssue(t, store, proj, "Deep Task", types.TypeTask)
	addRoadmapDep(t, store, descendant.ID, milestone.ID, types.DepParentChild)
	outsider := setupRoadmapIssue(t, store, proj, "Outside Task", types.TypeTask)

	issues, err := store.GetReadyWork(ctx, types.WorkFilter{ProjectID: proj.ID, Under: release.ID})
	if err != nil {
		t.Fatalf("GetReadyWork with Under failed: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("GetReadyWork with Under returned %d issues, want 1", len(issues))
	}
	if issues[0].ID != descendant.ID {
		t.Errorf("Under returned %s, want descendant %s", issues[0].ID, descendant.ID)
	}
	if _, ok := readyIDs(issues)[outsider.ID]; ok {
		t.Errorf("issue outside the subtree should not appear under %s", release.ID)
	}

	// Gating still applies inside the scoped subtree.
	setRoadmapStatus(t, store, milestone.ID, types.StatusDeferred)

	issues, err = store.GetReadyWork(ctx, types.WorkFilter{ProjectID: proj.ID, Under: milestone.ID})
	if err != nil {
		t.Fatalf("GetReadyWork under gated milestone failed: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("GetReadyWork under gated milestone returned %d issues, want 0", len(issues))
	}
}

func TestGetReadyWorkRoadmapCycleTerminates(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	a := setupRoadmapIssue(t, store, proj, "Cycle A", types.TypeTask)
	b := setupRoadmapIssue(t, store, proj, "Cycle B", types.TypeTask)

	// AddDependency rejects cycles, so write the bad edges directly.
	_, err := store.DB().ExecContext(ctx, `
INSERT INTO dependencies (issue_id, depends_on_id, type)
VALUES (?, ?, 'parent-child'), (?, ?, 'parent-child')`,
		a.ID, b.ID, b.ID, a.ID)
	if err != nil {
		t.Fatalf("inserting cyclic dependencies failed: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := store.GetReadyWork(ctx, types.WorkFilter{ProjectID: proj.ID})
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("GetReadyWork on cyclic hierarchy failed: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("GetReadyWork did not terminate on a cyclic hierarchy")
	}
}

func TestCloseRoadmapContainerWithOpenChildren(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	for _, containerType := range []types.IssueType{types.TypeRelease, types.TypeMilestone} {
		parent := setupRoadmapIssue(t, store, proj, "Container "+string(containerType), containerType)
		child := setupRoadmapIssue(t, store, proj, "Child of "+string(containerType), types.TypeTask)
		addRoadmapDep(t, store, child.ID, parent.ID, types.DepParentChild)

		err := store.CloseIssue(ctx, parent.ID, "done", false, "test-actor")
		var openChildErr *types.OpenChildrenError
		if !errors.As(err, &openChildErr) {
			t.Fatalf("CloseIssue(%s) error = %T (%v), want *types.OpenChildrenError", containerType, err, err)
		}
		if openChildErr.IssueID != parent.ID {
			t.Errorf("OpenChildrenError.IssueID = %s, want %s", openChildErr.IssueID, parent.ID)
		}
	}
}
