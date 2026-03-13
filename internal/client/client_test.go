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

func TestClientSetInlinePlan(t *testing.T) {
	client, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, client)
	issue := createTestIssueClient(t, client, proj.ID, "Issue with Plan")

	// Set inline plan
	planText := "This is the plan for the issue"
	comment, err := client.SetInlinePlan(proj.ID, issue.ID, planText)
	if err != nil {
		t.Fatalf("SetInlinePlan failed: %v", err)
	}

	if comment.Text != planText {
		t.Errorf("comment.Text = %q, want %q", comment.Text, planText)
	}
	if comment.CommentType != types.CommentTypePlan {
		t.Errorf("comment.CommentType = %q, want %q", comment.CommentType, types.CommentTypePlan)
	}
}

func TestClientGetPlanContext(t *testing.T) {
	client, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, client)
	issue := createTestIssueClient(t, client, proj.ID, "Issue for Context")

	// Initially no plan
	pc, err := client.GetPlanContext(proj.ID, issue.ID)
	if err != nil {
		t.Fatalf("GetPlanContext failed: %v", err)
	}
	if pc.InlinePlan != nil {
		t.Error("InlinePlan should be nil initially")
	}

	// Set inline plan
	planText := "The plan content"
	_, err = client.SetInlinePlan(proj.ID, issue.ID, planText)
	if err != nil {
		t.Fatalf("SetInlinePlan failed: %v", err)
	}

	// Get plan context again
	pc, err = client.GetPlanContext(proj.ID, issue.ID)
	if err != nil {
		t.Fatalf("GetPlanContext failed: %v", err)
	}
	if pc.InlinePlan == nil {
		t.Fatal("InlinePlan should not be nil after setting")
	}
	if pc.InlinePlan.Text != planText {
		t.Errorf("InlinePlan.Text = %q, want %q", pc.InlinePlan.Text, planText)
	}
}

func TestClientGetPlanHistory(t *testing.T) {
	client, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, client)
	issue := createTestIssueClient(t, client, proj.ID, "Issue with History")

	// Create multiple versions
	versions := []string{"V1", "V2", "V3"}
	for _, v := range versions {
		if _, err := client.SetInlinePlan(proj.ID, issue.ID, v); err != nil {
			t.Fatalf("SetInlinePlan failed: %v", err)
		}
	}

	// Get history
	history, err := client.GetPlanHistory(proj.ID, issue.ID)
	if err != nil {
		t.Fatalf("GetPlanHistory failed: %v", err)
	}

	if len(history) != len(versions) {
		t.Errorf("history length = %d, want %d", len(history), len(versions))
	}
}

func TestClientCreateAndGetPlan(t *testing.T) {
	client, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, client)

	// Create shared plan
	plan, err := client.CreatePlan(proj.ID, "Shared Plan", "Plan content")
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	if plan.Title != "Shared Plan" {
		t.Errorf("plan.Title = %q, want %q", plan.Title, "Shared Plan")
	}
	if plan.Content != "Plan content" {
		t.Errorf("plan.Content = %q, want %q", plan.Content, "Plan content")
	}

	// Get plan
	retrieved, err := client.GetPlan(proj.ID, plan.ID)
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}

	if retrieved.ID != plan.ID {
		t.Errorf("retrieved.ID = %q, want %q", retrieved.ID, plan.ID)
	}
}

func TestClientListPlans(t *testing.T) {
	client, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, client)

	// Create multiple plans
	for i := range 3 {
		_, err := client.CreatePlan(proj.ID, "Plan "+string(rune('A'+i)), "Content")
		if err != nil {
			t.Fatalf("CreatePlan failed: %v", err)
		}
	}

	// List plans
	plans, err := client.ListPlans(proj.ID)
	if err != nil {
		t.Fatalf("ListPlans failed: %v", err)
	}

	if len(plans) != 3 {
		t.Errorf("plans length = %d, want 3", len(plans))
	}
}

func TestClientUpdatePlan(t *testing.T) {
	client, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, client)

	// Create plan
	plan, err := client.CreatePlan(proj.ID, "Original", "Original content")
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	// Update plan
	updated, err := client.UpdatePlan(proj.ID, plan.ID, "Updated", "Updated content")
	if err != nil {
		t.Fatalf("UpdatePlan failed: %v", err)
	}

	if updated.Title != "Updated" {
		t.Errorf("updated.Title = %q, want %q", updated.Title, "Updated")
	}
	if updated.Content != "Updated content" {
		t.Errorf("updated.Content = %q, want %q", updated.Content, "Updated content")
	}
}

func TestClientDeletePlan(t *testing.T) {
	client, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, client)

	// Create plan
	plan, err := client.CreatePlan(proj.ID, "To Delete", "")
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	// Delete plan
	if err := client.DeletePlan(proj.ID, plan.ID); err != nil {
		t.Fatalf("DeletePlan failed: %v", err)
	}

	// Verify deletion
	_, err = client.GetPlan(proj.ID, plan.ID)
	if err == nil {
		t.Error("GetPlan should fail after deletion")
	}
}

func TestClientLinkAndUnlinkIssuesToPlan(t *testing.T) {
	client, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, client)
	issue := createTestIssueClient(t, client, proj.ID, "Issue to Link")

	// Create plan
	plan, err := client.CreatePlan(proj.ID, "Linkable", "")
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	// Link issue
	if err := client.LinkIssuesToPlan(proj.ID, plan.ID, []string{issue.ID}); err != nil {
		t.Fatalf("LinkIssuesToPlan failed: %v", err)
	}

	// Verify link
	retrieved, err := client.GetPlan(proj.ID, plan.ID)
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}
	if len(retrieved.LinkedIssues) != 1 {
		t.Errorf("LinkedIssues length = %d, want 1", len(retrieved.LinkedIssues))
	}

	// Unlink issue
	if err := client.UnlinkIssueFromPlan(proj.ID, plan.ID, issue.ID); err != nil {
		t.Fatalf("UnlinkIssueFromPlan failed: %v", err)
	}

	// Verify unlink
	retrieved, err = client.GetPlan(proj.ID, plan.ID)
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}
	if len(retrieved.LinkedIssues) != 0 {
		t.Errorf("LinkedIssues after unlink length = %d, want 0", len(retrieved.LinkedIssues))
	}
}

func TestClientCloseIssueSendsCascade(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)
	issue := createTestIssueClient(t, c, proj.ID, "Issue to close")

	// Close without cascade should work for issue with no children
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

	// Create parent epic
	parent := createTestIssueClient(t, c, proj.ID, "Parent epic")

	// Create open child under parent
	child, err := c.CreateIssue(proj.ID, client.CreateIssueRequest{
		Title:     "Open child",
		IssueType: "task",
		Priority:  2,
		ParentID:  parent.ID,
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	// Try to close parent without cascade - should get OpenChildrenError
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

	// Create parent epic
	parent := createTestIssueClient(t, c, proj.ID, "Parent epic")

	// Create open child under parent
	_, err := c.CreateIssue(proj.ID, client.CreateIssueRequest{
		Title:     "Open child",
		IssueType: "task",
		Priority:  2,
		ParentID:  parent.ID,
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	// Close parent with cascade - should succeed
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

	// Create a canonical workspace
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

	// Create a symlink workspace
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

	// Create a workspace without path_type — should default to canonical
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

// TestClientSetActor verifies that SetActor does not panic.
// The actor field is unexported; actual behavior is tested through integration tests.
func TestClientSetActor(t *testing.T) {
	c := client.New("http://localhost:8080")
	c.SetActor("test-user") // should not panic
}

// TestClientListIssuesParentFilter verifies that the Parent option on
// ListIssuesOptions causes the client to send the parent_id query parameter,
// filtering results to only children of the specified parent issue.
func TestClientMergeProjects(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	// Create target project
	target, err := c.CreateProject("Target", "tgt", "Target project")
	if err != nil {
		t.Fatalf("create target project: %v", err)
	}

	// Create source project with an issue
	source, err := c.CreateProject("Source", "src", "Source project")
	if err != nil {
		t.Fatalf("create source project: %v", err)
	}
	createTestIssueClient(t, c, source.ID, "Issue in source")

	// Merge source into target
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

	// Create an epic (parent)
	epic, err := c.CreateIssue(proj.ID, client.CreateIssueRequest{
		Title:     "Epic parent",
		IssueType: "epic",
		Priority:  1,
	})
	if err != nil {
		t.Fatalf("create epic: %v", err)
	}

	// Create a child under the epic
	child, err := c.CreateIssue(proj.ID, client.CreateIssueRequest{
		Title:     "Child task",
		IssueType: "task",
		Priority:  2,
		ParentID:  epic.ID,
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	// Create an unrelated issue (no parent)
	_, err = c.CreateIssue(proj.ID, client.CreateIssueRequest{
		Title:     "Unrelated task",
		IssueType: "task",
		Priority:  2,
	})
	if err != nil {
		t.Fatalf("create unrelated: %v", err)
	}

	// List with Parent filter should only return children of the epic
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

	// List without Parent filter should return all issues
	allIssues, err := c.ListIssues(proj.ID, client.ListIssuesOptions{})
	if err != nil {
		t.Fatalf("ListIssues without Parent: %v", err)
	}
	if len(allIssues) < 2 {
		t.Errorf("expected at least 2 issues without parent filter, got %d", len(allIssues))
	}
}
