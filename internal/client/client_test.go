package client_test

import (
	"errors"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/arc/internal/api"
	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
)

// intPtr returns a pointer to the given int, for building CreateIssueRequest
// literals where Priority must be a *int.
func intPtr(i int) *int {
	return &i
}

// createTestPlanClient creates a plan for testing via client and returns its ID.
func testClientServer(t *testing.T) (*client.Client, func()) {
	t.Helper()

	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	server := api.New(api.ServerOptions{
		Address: ":0",
		Store:   store,
	})

	ts := httptest.NewServer(server.Echo())

	c := client.New(ts.URL)
	c.SetActor("test-user")

	cleanup := func() {
		ts.Close()
		_ = store.Close()
	}

	return c, cleanup
}

// createTestProjectClient creates a project for testing via client.
func createTestProjectClient(t *testing.T, c *client.Client) *types.Project {
	t.Helper()

	proj, err := c.CreateProject("Test Project", "test", "Test description")
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	return proj
}

// createTestIssueClient creates an issue for testing via client.
func createTestIssueClient(t *testing.T, c *client.Client, projID, title string) *types.Issue {
	t.Helper()

	issue, err := c.CreateIssue(projID, client.CreateIssueRequest{
		Title:     title,
		IssueType: "task",
		Priority:  intPtr(2),
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	return issue
}

// --- Non-plan tests ---

// TestClientCreateIssuePreservesPriorityZero pins the fix for a bug where an
// explicit priority of 0 (critical) was clobbered to 2 on create. It exercises
// the full path: client request -> API handler -> storage.
func TestClientCreateIssuePreservesPriorityZero(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	issue, err := c.CreateIssue(proj.ID, client.CreateIssueRequest{
		Title:     "Critical issue",
		IssueType: "task",
		Priority:  intPtr(0),
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	if issue.Priority != 0 {
		t.Errorf("Priority = %d, want 0", issue.Priority)
	}
}

// TestClientCreateIssueDefaultsPriorityWhenOmitted verifies that a create
// request with no Priority set still defaults to priority 2.
func TestClientCreateIssueDefaultsPriorityWhenOmitted(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	issue, err := c.CreateIssue(proj.ID, client.CreateIssueRequest{
		Title:     "No priority set",
		IssueType: "task",
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	if issue.Priority != 2 {
		t.Errorf("Priority = %d, want 2", issue.Priority)
	}
}

func TestClientCloseIssueSendsCascade(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)
	issue := createTestIssueClient(t, c, proj.ID, "Issue to close")

	closed, err := c.CloseIssue(proj.ID, issue.ID, "done", false)
	if err != nil {
		t.Fatalf("CloseIssue failed: %v", err)
	}
	if closed.Status != types.StatusClosed {
		t.Errorf("status = %q, want %q", closed.Status, types.StatusClosed)
	}
}

func TestClientCloseIssueReturnsOpenChildrenError(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	parent := createTestIssueClient(t, c, proj.ID, "Parent epic")

	child, err := c.CreateIssue(proj.ID, client.CreateIssueRequest{
		Title:     "Open child",
		IssueType: "task",
		Priority:  intPtr(2),
		ParentID:  parent.ID,
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	_, err = c.CloseIssue(proj.ID, parent.ID, "done", false)
	if err == nil {
		t.Fatal("expected error when closing parent with open children")
	}

	var openChildrenErr *types.OpenChildrenError
	if !errors.As(err, &openChildrenErr) {
		t.Fatalf("expected *types.OpenChildrenError, got %T: %v", err, err)
	}

	if openChildrenErr.IssueID != parent.ID {
		t.Errorf("IssueID = %q, want %q", openChildrenErr.IssueID, parent.ID)
	}
	if len(openChildrenErr.Children) != 1 {
		t.Fatalf("expected 1 open child, got %d", len(openChildrenErr.Children))
	}
	if openChildrenErr.Children[0].ID != child.ID {
		t.Errorf("child ID = %q, want %q", openChildrenErr.Children[0].ID, child.ID)
	}
}

func TestClientCloseIssueWithCascade(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	parent := createTestIssueClient(t, c, proj.ID, "Parent epic")

	_, err := c.CreateIssue(proj.ID, client.CreateIssueRequest{
		Title:     "Open child",
		IssueType: "task",
		Priority:  intPtr(2),
		ParentID:  parent.ID,
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	closed, err := c.CloseIssue(proj.ID, parent.ID, "done", true)
	if err != nil {
		t.Fatalf("CloseIssue with cascade failed: %v", err)
	}
	if closed.Status != types.StatusClosed {
		t.Errorf("status = %q, want %q", closed.Status, types.StatusClosed)
	}
}

func TestClientHealth(t *testing.T) {
	client, cleanup := testClientServer(t)
	defer cleanup()

	if err := client.Health(); err != nil {
		t.Errorf("Health check failed: %v", err)
	}
}

func TestClientCreateWorkspaceWithPathType(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	canonical, err := c.CreateWorkspace(proj.ID, client.CreateWorkspaceRequest{
		Path:     "/Volumes/ExternalSSD/project",
		Label:    "project",
		PathType: "canonical",
	})
	if err != nil {
		t.Fatalf("CreateWorkspace(canonical) failed: %v", err)
	}
	if canonical.PathType != "canonical" {
		t.Errorf("PathType = %q, want %q", canonical.PathType, "canonical")
	}

	symlink, err := c.CreateWorkspace(proj.ID, client.CreateWorkspaceRequest{
		Path:     "/Users/dev/project",
		Label:    "project",
		PathType: "symlink",
	})
	if err != nil {
		t.Fatalf("CreateWorkspace(symlink) failed: %v", err)
	}
	if symlink.PathType != "symlink" {
		t.Errorf("PathType = %q, want %q", symlink.PathType, "symlink")
	}

	defaultWs, err := c.CreateWorkspace(proj.ID, client.CreateWorkspaceRequest{
		Path:  "/home/user/project",
		Label: "project-default",
	})
	if err != nil {
		t.Fatalf("CreateWorkspace(default) failed: %v", err)
	}
	if defaultWs.PathType != "canonical" {
		t.Errorf("PathType = %q, want %q", defaultWs.PathType, "canonical")
	}
}

func TestClientSetActor(t *testing.T) {
	c := client.New("http://localhost:8080")
	c.SetActor("test-user")
}

func TestClientMergeProjects(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	target, err := c.CreateProject("Target", "tgt", "Target project")
	if err != nil {
		t.Fatalf("create target project: %v", err)
	}

	source, err := c.CreateProject("Source", "src", "Source project")
	if err != nil {
		t.Fatalf("create source project: %v", err)
	}
	createTestIssueClient(t, c, source.ID, "Issue in source")

	result, err := c.MergeProjects(target.ID, []string{source.ID})
	if err != nil {
		t.Fatalf("MergeProjects failed: %v", err)
	}

	if result.TargetProject.ID != target.ID {
		t.Errorf("TargetProject.ID = %q, want %q", result.TargetProject.ID, target.ID)
	}
	if result.IssuesMoved != 1 {
		t.Errorf("IssuesMoved = %d, want 1", result.IssuesMoved)
	}
	if len(result.SourcesDeleted) != 1 {
		t.Errorf("SourcesDeleted length = %d, want 1", len(result.SourcesDeleted))
	}
}

func TestClientListIssuesParentFilter(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	epic, err := c.CreateIssue(proj.ID, client.CreateIssueRequest{
		Title:     "Epic parent",
		IssueType: "epic",
		Priority:  intPtr(1),
	})
	if err != nil {
		t.Fatalf("create epic: %v", err)
	}

	child, err := c.CreateIssue(proj.ID, client.CreateIssueRequest{
		Title:     "Child task",
		IssueType: "task",
		Priority:  intPtr(2),
		ParentID:  epic.ID,
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	_, err = c.CreateIssue(proj.ID, client.CreateIssueRequest{
		Title:     "Unrelated task",
		IssueType: "task",
		Priority:  intPtr(2),
	})
	if err != nil {
		t.Fatalf("create unrelated: %v", err)
	}

	issues, err := c.ListIssues(proj.ID, client.ListIssuesOptions{
		Parent: epic.ID,
	})
	if err != nil {
		t.Fatalf("ListIssues with Parent: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("expected 1 child issue, got %d", len(issues))
	}
	if issues[0].ID != child.ID {
		t.Errorf("expected child ID %q, got %q", child.ID, issues[0].ID)
	}

	allIssues, err := c.ListIssues(proj.ID, client.ListIssuesOptions{})
	if err != nil {
		t.Fatalf("ListIssues without Parent: %v", err)
	}
	if len(allIssues) < 2 {
		t.Errorf("expected at least 2 issues without parent filter, got %d", len(allIssues))
	}
}

func TestClientLegacyPlansRequireUpgrade(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()
	cases := []func() error{
		func() error { _, err := c.CreatePlan("/unavailable/client.md"); return err },
		func() error { _, err := c.GetPlan("plan.old"); return err },
		func() error { return c.UpdatePlanContent("plan.old", "content") },
		func() error { return c.UpdatePlanStatus("plan.old", "approved") },
		func() error { return c.DeletePlan("plan.old") },
		func() error { _, err := c.ListPlanComments("plan.old"); return err },
		func() error { _, err := c.CreatePlanComment("plan.old", nil, "feedback"); return err },
		func() error {
			_, err := c.UpdatePlanComment("plan.old", "pc.old", client.UpdatePlanCommentRequest{})
			return err
		},
		func() error { return c.DeletePlanComment("plan.old", "pc.old") },
	}
	for _, call := range cases {
		if err := call(); !errors.Is(err, client.ErrPlanUpgrade) {
			t.Fatalf("expected upgrade error: %v", err)
		}
	}
}

func TestClientListIssuesPriorityFilter(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	p0, err := c.CreateIssue(proj.ID, client.CreateIssueRequest{
		Title: "P0 task", IssueType: "task", Priority: intPtr(0),
	})
	if err != nil {
		t.Fatalf("create p0: %v", err)
	}
	_, err = c.CreateIssue(proj.ID, client.CreateIssueRequest{
		Title: "P1 task", IssueType: "task", Priority: intPtr(1),
	})
	if err != nil {
		t.Fatalf("create p1: %v", err)
	}
	p2, err := c.CreateIssue(proj.ID, client.CreateIssueRequest{
		Title: "P2 task", IssueType: "task", Priority: intPtr(2),
	})
	if err != nil {
		t.Fatalf("create p2: %v", err)
	}

	only0, err := c.ListIssues(proj.ID, client.ListIssuesOptions{Priorities: []int{0}})
	if err != nil {
		t.Fatalf("ListIssues with Priorities {0}: %v", err)
	}
	if len(only0) != 1 || only0[0].ID != p0.ID {
		t.Errorf("expected only the P0 issue, got %d issues", len(only0))
	}

	both, err := c.ListIssues(proj.ID, client.ListIssuesOptions{Priorities: []int{0, 2}})
	if err != nil {
		t.Fatalf("ListIssues with Priorities {0,2}: %v", err)
	}
	if len(both) != 2 {
		t.Fatalf("expected 2 issues for priorities {0,2}, got %d", len(both))
	}
	gotIDs := map[string]bool{both[0].ID: true, both[1].ID: true}
	if !gotIDs[p0.ID] || !gotIDs[p2.ID] {
		t.Errorf("expected P0 and P2 issues, got %v", gotIDs)
	}

	all, err := c.ListIssues(proj.ID, client.ListIssuesOptions{})
	if err != nil {
		t.Fatalf("ListIssues without priorities: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 issues without priority filter, got %d", len(all))
	}
}
