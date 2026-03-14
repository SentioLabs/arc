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

// testClientServer creates a test server and client for testing.
func testClientServer(t *testing.T) (*client.Client, func()) {
	t.Helper()

	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	server := api.New(api.Config{
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
		Priority:  2,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	return issue
}

// --- Plan tests ---

func TestClientSetPlan(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)
	issue := createTestIssueClient(t, c, proj.ID, "Issue with Plan")

	plan, err := c.SetPlan(proj.ID, issue.ID, "This is the plan", "draft")
	if err != nil {
		t.Fatalf("SetPlan failed: %v", err)
	}

	if plan.Content != "This is the plan" {
		t.Errorf("plan.Content = %q, want %q", plan.Content, "This is the plan")
	}
	if plan.Status != types.PlanStatusDraft {
		t.Errorf("plan.Status = %q, want %q", plan.Status, types.PlanStatusDraft)
	}
	if plan.IssueID != issue.ID {
		t.Errorf("plan.IssueID = %q, want %q", plan.IssueID, issue.ID)
	}
}

func TestClientSetPlanDefaultStatus(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)
	issue := createTestIssueClient(t, c, proj.ID, "Issue default status")

	// Empty status should default to draft
	plan, err := c.SetPlan(proj.ID, issue.ID, "Plan text", "")
	if err != nil {
		t.Fatalf("SetPlan failed: %v", err)
	}

	if plan.Status != types.PlanStatusDraft {
		t.Errorf("plan.Status = %q, want %q", plan.Status, types.PlanStatusDraft)
	}
}

func TestClientGetPlanByIssue(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)
	issue := createTestIssueClient(t, c, proj.ID, "Issue for GetPlanByIssue")

	// Set a plan first
	_, err := c.SetPlan(proj.ID, issue.ID, "The plan content", "draft")
	if err != nil {
		t.Fatalf("SetPlan failed: %v", err)
	}

	// Get it back
	plan, err := c.GetPlanByIssue(proj.ID, issue.ID)
	if err != nil {
		t.Fatalf("GetPlanByIssue failed: %v", err)
	}

	if plan.Content != "The plan content" {
		t.Errorf("plan.Content = %q, want %q", plan.Content, "The plan content")
	}
	if plan.IssueID != issue.ID {
		t.Errorf("plan.IssueID = %q, want %q", plan.IssueID, issue.ID)
	}
}

func TestClientGetPlanByIssueNotFound(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)
	issue := createTestIssueClient(t, c, proj.ID, "Issue no plan")

	// Should return error when no plan exists
	_, err := c.GetPlanByIssue(proj.ID, issue.ID)
	if err == nil {
		t.Error("expected error for issue with no plan")
	}
}

func TestClientListPlans(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	// Create plans via issues
	for i := range 3 {
		issue := createTestIssueClient(t, c, proj.ID, "Issue "+string(rune('A'+i)))
		_, err := c.SetPlan(proj.ID, issue.ID, "Plan "+string(rune('A'+i)), "draft")
		if err != nil {
			t.Fatalf("SetPlan failed: %v", err)
		}
	}

	// List all plans (no status filter)
	plans, err := c.ListPlans(proj.ID, "")
	if err != nil {
		t.Fatalf("ListPlans failed: %v", err)
	}

	if len(plans) != 3 {
		t.Errorf("plans length = %d, want 3", len(plans))
	}
}

func TestClientListPlansWithStatusFilter(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	// Create a draft plan
	issue1 := createTestIssueClient(t, c, proj.ID, "Draft issue")
	_, err := c.SetPlan(proj.ID, issue1.ID, "Draft plan", "draft")
	if err != nil {
		t.Fatalf("SetPlan failed: %v", err)
	}

	// Create an approved plan
	issue2 := createTestIssueClient(t, c, proj.ID, "Approved issue")
	p2, err := c.SetPlan(proj.ID, issue2.ID, "Approved plan", "draft")
	if err != nil {
		t.Fatalf("SetPlan failed: %v", err)
	}
	// Update status to approved
	if err := c.UpdatePlanStatus(proj.ID, p2.ID, "approved"); err != nil {
		t.Fatalf("UpdatePlanStatus failed: %v", err)
	}

	// List only draft plans
	drafts, err := c.ListPlans(proj.ID, "draft")
	if err != nil {
		t.Fatalf("ListPlans(draft) failed: %v", err)
	}

	if len(drafts) != 1 {
		t.Errorf("draft plans = %d, want 1", len(drafts))
	}
}

func TestClientGetPlan(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)
	issue := createTestIssueClient(t, c, proj.ID, "Issue for GetPlan")

	created, err := c.SetPlan(proj.ID, issue.ID, "Plan content", "draft")
	if err != nil {
		t.Fatalf("SetPlan failed: %v", err)
	}

	// Get plan by plan ID
	plan, err := c.GetPlan(proj.ID, created.ID)
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}

	if plan.ID != created.ID {
		t.Errorf("plan.ID = %q, want %q", plan.ID, created.ID)
	}
	if plan.Content != "Plan content" {
		t.Errorf("plan.Content = %q, want %q", plan.Content, "Plan content")
	}
}

func TestClientUpdatePlanStatus(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)
	issue := createTestIssueClient(t, c, proj.ID, "Issue for status update")

	plan, err := c.SetPlan(proj.ID, issue.ID, "Plan text", "draft")
	if err != nil {
		t.Fatalf("SetPlan failed: %v", err)
	}

	// Update status to approved
	if err := c.UpdatePlanStatus(proj.ID, plan.ID, "approved"); err != nil {
		t.Fatalf("UpdatePlanStatus failed: %v", err)
	}

	// Verify
	updated, err := c.GetPlan(proj.ID, plan.ID)
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}
	if updated.Status != types.PlanStatusApproved {
		t.Errorf("status = %q, want %q", updated.Status, types.PlanStatusApproved)
	}
}

func TestClientUpdatePlanContent(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)
	issue := createTestIssueClient(t, c, proj.ID, "Issue for content update")

	plan, err := c.SetPlan(proj.ID, issue.ID, "Original content", "draft")
	if err != nil {
		t.Fatalf("SetPlan failed: %v", err)
	}

	// Update content
	if err := c.UpdatePlanContent(proj.ID, plan.ID, "New Title", "New content"); err != nil {
		t.Fatalf("UpdatePlanContent failed: %v", err)
	}

	// Verify
	updated, err := c.GetPlan(proj.ID, plan.ID)
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}
	if updated.Title != "New Title" {
		t.Errorf("title = %q, want %q", updated.Title, "New Title")
	}
	if updated.Content != "New content" {
		t.Errorf("content = %q, want %q", updated.Content, "New content")
	}
}

func TestClientDeletePlan(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)
	issue := createTestIssueClient(t, c, proj.ID, "Issue for delete")

	plan, err := c.SetPlan(proj.ID, issue.ID, "Plan to delete", "draft")
	if err != nil {
		t.Fatalf("SetPlan failed: %v", err)
	}

	// Delete
	if err := c.DeletePlan(proj.ID, plan.ID); err != nil {
		t.Fatalf("DeletePlan failed: %v", err)
	}

	// Verify deletion
	_, err = c.GetPlan(proj.ID, plan.ID)
	if err == nil {
		t.Error("GetPlan should fail after deletion")
	}
}

func TestClientGetPendingPlanCount(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	// Initially zero
	count, err := c.GetPendingPlanCount(proj.ID)
	if err != nil {
		t.Fatalf("GetPendingPlanCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	// Create some draft plans
	for i := range 2 {
		issue := createTestIssueClient(t, c, proj.ID, "Pending issue "+string(rune('A'+i)))
		_, err := c.SetPlan(proj.ID, issue.ID, "Draft plan", "draft")
		if err != nil {
			t.Fatalf("SetPlan failed: %v", err)
		}
	}

	count, err = c.GetPendingPlanCount(proj.ID)
	if err != nil {
		t.Fatalf("GetPendingPlanCount failed: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

// --- Non-plan tests (kept from original) ---

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
		Priority:  2,
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
		Priority:  2,
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
		Priority:  1,
	})
	if err != nil {
		t.Fatalf("create epic: %v", err)
	}

	child, err := c.CreateIssue(proj.ID, client.CreateIssueRequest{
		Title:     "Child task",
		IssueType: "task",
		Priority:  2,
		ParentID:  epic.ID,
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	_, err = c.CreateIssue(proj.ID, client.CreateIssueRequest{
		Title:     "Unrelated task",
		IssueType: "task",
		Priority:  2,
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
