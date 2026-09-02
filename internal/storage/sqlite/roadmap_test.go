// This file lives in package sqlite (not sqlite_test) so it can reach
// readyBaseSQL, an unexported helper, for the container-type tripwire test.
package sqlite //nolint:testpackage // needs readyBaseSQL, an unexported helper

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sentiolabs/arc/internal/types"
)

// newRoadmapTestStore creates a temporary store for roadmap tests.
func newRoadmapTestStore(t *testing.T) (*Store, func()) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	return store, func() { _ = store.Close() }
}

// newRoadmapTestProject creates a project for roadmap tests.
func newRoadmapTestProject(t *testing.T, store *Store) *types.Project {
	t.Helper()

	proj := &types.Project{Name: "Roadmap Test Project", Prefix: "rm"}
	if err := store.CreateProject(context.Background(), proj); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}
	return proj
}

// newRoadmapTestIssue creates an open issue of the given type.
func newRoadmapTestIssue(
	t *testing.T, store *Store, proj *types.Project, title string, issueType types.IssueType,
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

// linkRoadmapIssues adds a dependency between two issues.
func linkRoadmapIssues(t *testing.T, store *Store, issueID, dependsOnID string, depType types.DependencyType) {
	t.Helper()

	dep := &types.Dependency{IssueID: issueID, DependsOnID: dependsOnID, Type: depType}
	if err := store.AddDependency(context.Background(), dep, "test-actor"); err != nil {
		t.Fatalf("AddDependency(%s -> %s, %s) failed: %v", issueID, dependsOnID, depType, err)
	}
}

// findRoadmapNode searches a roadmap tree (and all descendants) for a node by issue ID.
func findRoadmapNode(nodes []*types.RoadmapNode, id string) *types.RoadmapNode {
	for _, n := range nodes {
		if n.Issue.ID == id {
			return n
		}
		if found := findRoadmapNode(n.Children, id); found != nil {
			return found
		}
	}
	return nil
}

func TestGetRoadmap(t *testing.T) {
	store, cleanup := newRoadmapTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := newRoadmapTestProject(t, store)

	release := newRoadmapTestIssue(t, store, proj, "v1", types.TypeRelease)
	if err := store.UpdateIssue(ctx, release.ID, map[string]any{"priority": 0}, "test-actor"); err != nil {
		t.Fatalf("UpdateIssue(release priority) failed: %v", err)
	}

	m1 := newRoadmapTestIssue(t, store, proj, "M1", types.TypeMilestone)
	linkRoadmapIssues(t, store, m1.ID, release.ID, types.DepParentChild)

	m2 := newRoadmapTestIssue(t, store, proj, "M2", types.TypeMilestone)
	linkRoadmapIssues(t, store, m2.ID, release.ID, types.DepParentChild)
	linkRoadmapIssues(t, store, m2.ID, m1.ID, types.DepBlocks)

	e1 := newRoadmapTestIssue(t, store, proj, "E1", types.TypeEpic)
	linkRoadmapIssues(t, store, e1.ID, m1.ID, types.DepParentChild)

	t1 := newRoadmapTestIssue(t, store, proj, "t1", types.TypeTask)
	linkRoadmapIssues(t, store, t1.ID, e1.ID, types.DepParentChild)
	if err := store.CloseIssue(ctx, t1.ID, "done", false, "test-actor"); err != nil {
		t.Fatalf("CloseIssue(t1) failed: %v", err)
	}

	t2 := newRoadmapTestIssue(t, store, proj, "t2", types.TypeTask)
	linkRoadmapIssues(t, store, t2.ID, e1.ID, types.DepParentChild)

	standalone := newRoadmapTestIssue(t, store, proj, "standalone", types.TypeTask)

	nodes, err := store.GetRoadmap(ctx, proj.ID)
	if err != nil {
		t.Fatalf("GetRoadmap failed: %v", err)
	}

	if len(nodes) != 1 {
		t.Fatalf("expected 1 root, got %d", len(nodes))
	}
	if nodes[0].Issue.ID != release.ID {
		t.Fatalf("expected root %s, got %s", release.ID, nodes[0].Issue.ID)
	}

	v1Node := nodes[0]
	if v1Node.TotalCount != 2 {
		t.Errorf("v1.TotalCount = %d, want 2", v1Node.TotalCount)
	}
	if v1Node.ClosedCount != 1 {
		t.Errorf("v1.ClosedCount = %d, want 1", v1Node.ClosedCount)
	}

	m1Node := findRoadmapNode(nodes, m1.ID)
	if m1Node == nil {
		t.Fatalf("M1 node not found")
	}
	if len(m1Node.GatedBy) != 0 {
		t.Errorf("M1.GatedBy = %v, want empty", m1Node.GatedBy)
	}

	m2Node := findRoadmapNode(nodes, m2.ID)
	if m2Node == nil {
		t.Fatalf("M2 node not found")
	}
	if len(m2Node.GatedBy) != 1 || m2Node.GatedBy[0] != m1.ID {
		t.Errorf("M2.GatedBy = %v, want [%s]", m2Node.GatedBy, m1.ID)
	}
	if len(m2Node.Children) != 0 {
		t.Errorf("M2.Children = %v, want none", m2Node.Children)
	}

	if findRoadmapNode(nodes, standalone.ID) != nil {
		t.Errorf("standalone task should not appear in the roadmap tree")
	}
}

// TestGetRoadmapPopulatesLabels guards against roadmap nodes coming back
// without labels, which would hide the `parallel` marker the CLI renderer
// looks for on milestone nodes.
func TestGetRoadmapPopulatesLabels(t *testing.T) {
	store, cleanup := newRoadmapTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := newRoadmapTestProject(t, store)

	release := newRoadmapTestIssue(t, store, proj, "v1", types.TypeRelease)

	m1 := newRoadmapTestIssue(t, store, proj, "M1", types.TypeMilestone)
	linkRoadmapIssues(t, store, m1.ID, release.ID, types.DepParentChild)

	label := &types.Label{Name: "parallel"}
	if err := store.CreateLabel(ctx, label); err != nil {
		t.Fatalf("CreateLabel failed: %v", err)
	}
	if err := store.AddLabelToIssue(ctx, m1.ID, "parallel", "test-actor"); err != nil {
		t.Fatalf("AddLabelToIssue failed: %v", err)
	}

	nodes, err := store.GetRoadmap(ctx, proj.ID)
	if err != nil {
		t.Fatalf("GetRoadmap failed: %v", err)
	}

	m1Node := findRoadmapNode(nodes, m1.ID)
	if m1Node == nil {
		t.Fatalf("M1 node not found")
	}

	found := false
	for _, l := range m1Node.Issue.Labels {
		if l == "parallel" {
			found = true
		}
	}
	if !found {
		t.Errorf("M1.Issue.Labels = %v, want to include %q", m1Node.Issue.Labels, "parallel")
	}
}

func TestGetRoadmapEmptyProject(t *testing.T) {
	store, cleanup := newRoadmapTestStore(t)
	defer cleanup()

	proj := newRoadmapTestProject(t, store)

	nodes, err := store.GetRoadmap(context.Background(), proj.ID)
	if err != nil {
		t.Fatalf("GetRoadmap failed: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected empty roadmap, got %d nodes", len(nodes))
	}
}

// TestListIssuesDefaultFilterIncludesRoadmapTypes guards against release and
// milestone issues silently dropping out of unfiltered listings, which is
// what would happen if allIssueTypes forgot about the new container types.
func TestListIssuesDefaultFilterIncludesRoadmapTypes(t *testing.T) {
	store, cleanup := newRoadmapTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := newRoadmapTestProject(t, store)

	release := newRoadmapTestIssue(t, store, proj, "v1", types.TypeRelease)

	issues, err := store.ListIssues(ctx, types.IssueFilter{ProjectID: proj.ID})
	if err != nil {
		t.Fatalf("ListIssues failed: %v", err)
	}

	found := false
	for _, issue := range issues {
		if issue.ID == release.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("release issue %s missing from unfiltered ListIssues", release.ID)
	}
}

// TestGetRoadmapDiamondCountsOnce guards against double-counting a
// descendant reachable through two parent-child paths (a task under two
// sibling milestones of the same release). The release's TotalCount must
// count the task once, not once per path.
func TestGetRoadmapDiamondCountsOnce(t *testing.T) {
	store, cleanup := newRoadmapTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := newRoadmapTestProject(t, store)

	release := newRoadmapTestIssue(t, store, proj, "v1", types.TypeRelease)

	m1 := newRoadmapTestIssue(t, store, proj, "M1", types.TypeMilestone)
	linkRoadmapIssues(t, store, m1.ID, release.ID, types.DepParentChild)

	m2 := newRoadmapTestIssue(t, store, proj, "M2", types.TypeMilestone)
	linkRoadmapIssues(t, store, m2.ID, release.ID, types.DepParentChild)

	shared := newRoadmapTestIssue(t, store, proj, "shared task", types.TypeTask)
	linkRoadmapIssues(t, store, shared.ID, m1.ID, types.DepParentChild)
	linkRoadmapIssues(t, store, shared.ID, m2.ID, types.DepParentChild)

	nodes, err := store.GetRoadmap(ctx, proj.ID)
	if err != nil {
		t.Fatalf("GetRoadmap failed: %v", err)
	}

	releaseNode := findRoadmapNode(nodes, release.ID)
	if releaseNode == nil {
		t.Fatalf("release node not found")
	}
	if releaseNode.TotalCount != 1 {
		t.Errorf("release.TotalCount = %d, want 1 (diamond task counted twice)", releaseNode.TotalCount)
	}
}

// TestGetRoadmapCycleTerminates guards against an infinite DFS when two
// containers point at each other via parent-child edges.
func TestGetRoadmapCycleTerminates(t *testing.T) {
	store, cleanup := newRoadmapTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := newRoadmapTestProject(t, store)

	a := newRoadmapTestIssue(t, store, proj, "Container A", types.TypeMilestone)
	b := newRoadmapTestIssue(t, store, proj, "Container B", types.TypeMilestone)

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
		_, err := store.GetRoadmap(ctx, proj.ID)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("GetRoadmap on cyclic hierarchy failed: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("GetRoadmap did not terminate on a cyclic hierarchy")
	}
}

// TestGetRoadmapGatedByCrossProjectBlocker guards against dropping open
// blockers that live in a different project than the gated issue, since
// loadRoadmapEdges only loads issues from the roadmap's own project.
func TestGetRoadmapGatedByCrossProjectBlocker(t *testing.T) {
	store, cleanup := newRoadmapTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := newRoadmapTestProject(t, store)
	otherProj := &types.Project{Name: "Other Roadmap Test Project", Prefix: "rmo"}
	if err := store.CreateProject(ctx, otherProj); err != nil {
		t.Fatalf("CreateProject(otherProj) failed: %v", err)
	}

	milestone := newRoadmapTestIssue(t, store, proj, "M1", types.TypeMilestone)
	blocker := newRoadmapTestIssue(t, store, otherProj, "blocker", types.TypeTask)
	linkRoadmapIssues(t, store, milestone.ID, blocker.ID, types.DepBlocks)

	nodes, err := store.GetRoadmap(ctx, proj.ID)
	if err != nil {
		t.Fatalf("GetRoadmap failed: %v", err)
	}

	m1Node := findRoadmapNode(nodes, milestone.ID)
	if m1Node == nil {
		t.Fatalf("M1 node not found")
	}
	found := false
	for _, id := range m1Node.GatedBy {
		if id == blocker.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("M1.GatedBy = %v, want to include cross-project blocker %s", m1Node.GatedBy, blocker.ID)
	}
}

// TestReadyBaseSQLExcludesExactlyContainerTypes is a tripwire: if a future
// issue type becomes a container (IsContainer() == true) without updating
// readyBaseSQL's NOT IN literal, this test fails loudly instead of letting
// the new container type leak into `arc ready`.
func TestReadyBaseSQLExcludesExactlyContainerTypes(t *testing.T) {
	sql := readyBaseSQL(types.SortPolicyPriority, false)

	re := regexp.MustCompile(`i\.issue_type NOT IN \(([^)]*)\)`)
	match := re.FindStringSubmatch(sql)
	if match == nil {
		t.Fatalf("could not find issue_type NOT IN clause in readyBaseSQL output:\n%s", sql)
	}
	clause := match[1]

	for _, it := range types.AllIssueTypes() {
		literal := "'" + string(it) + "'"
		contains := strings.Contains(clause, literal)
		switch {
		case it.IsContainer() && !contains:
			t.Errorf("container type %q missing from NOT IN clause: %s", it, clause)
		case !it.IsContainer() && contains:
			t.Errorf("non-container type %q unexpectedly present in NOT IN clause: %s", it, clause)
		}
	}
}
