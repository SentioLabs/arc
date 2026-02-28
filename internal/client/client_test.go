package client_test

import (
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

// createTestWorkspaceClient creates a workspace for testing via client.
func createTestWorkspaceClient(t *testing.T, c *client.Client) *types.Workspace {
	t.Helper()

	ws, err := c.CreateWorkspace("Test Workspace", "test", "/tmp/test", "Test description")
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	return ws
}

// createTestIssueClient creates an issue for testing via client.
func createTestIssueClient(t *testing.T, c *client.Client, wsID, title string) *types.Issue {
	t.Helper()

	issue, err := c.CreateIssue(wsID, client.CreateIssueRequest{
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

	ws := createTestWorkspaceClient(t, client)
	issue := createTestIssueClient(t, client, ws.ID, "Issue with Plan")

	// Set inline plan
	planText := "This is the plan for the issue"
	comment, err := client.SetInlinePlan(ws.ID, issue.ID, planText)
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

	ws := createTestWorkspaceClient(t, client)
	issue := createTestIssueClient(t, client, ws.ID, "Issue for Context")

	// Initially no plan
	pc, err := client.GetPlanContext(ws.ID, issue.ID)
	if err != nil {
		t.Fatalf("GetPlanContext failed: %v", err)
	}
	if pc.InlinePlan != nil {
		t.Error("InlinePlan should be nil initially")
	}

	// Set inline plan
	planText := "The plan content"
	_, err = client.SetInlinePlan(ws.ID, issue.ID, planText)
	if err != nil {
		t.Fatalf("SetInlinePlan failed: %v", err)
	}

	// Get plan context again
	pc, err = client.GetPlanContext(ws.ID, issue.ID)
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

	ws := createTestWorkspaceClient(t, client)
	issue := createTestIssueClient(t, client, ws.ID, "Issue with History")

	// Create multiple versions
	versions := []string{"V1", "V2", "V3"}
	for _, v := range versions {
		if _, err := client.SetInlinePlan(ws.ID, issue.ID, v); err != nil {
			t.Fatalf("SetInlinePlan failed: %v", err)
		}
	}

	// Get history
	history, err := client.GetPlanHistory(ws.ID, issue.ID)
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

	ws := createTestWorkspaceClient(t, client)

	// Create shared plan
	plan, err := client.CreatePlan(ws.ID, "Shared Plan", "Plan content")
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
	retrieved, err := client.GetPlan(ws.ID, plan.ID)
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

	ws := createTestWorkspaceClient(t, client)

	// Create multiple plans
	for i := range 3 {
		_, err := client.CreatePlan(ws.ID, "Plan "+string(rune('A'+i)), "Content")
		if err != nil {
			t.Fatalf("CreatePlan failed: %v", err)
		}
	}

	// List plans
	plans, err := client.ListPlans(ws.ID)
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

	ws := createTestWorkspaceClient(t, client)

	// Create plan
	plan, err := client.CreatePlan(ws.ID, "Original", "Original content")
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	// Update plan
	updated, err := client.UpdatePlan(ws.ID, plan.ID, "Updated", "Updated content")
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

	ws := createTestWorkspaceClient(t, client)

	// Create plan
	plan, err := client.CreatePlan(ws.ID, "To Delete", "")
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	// Delete plan
	if err := client.DeletePlan(ws.ID, plan.ID); err != nil {
		t.Fatalf("DeletePlan failed: %v", err)
	}

	// Verify deletion
	_, err = client.GetPlan(ws.ID, plan.ID)
	if err == nil {
		t.Error("GetPlan should fail after deletion")
	}
}

func TestClientLinkAndUnlinkIssuesToPlan(t *testing.T) {
	client, cleanup := testClientServer(t)
	defer cleanup()

	ws := createTestWorkspaceClient(t, client)
	issue := createTestIssueClient(t, client, ws.ID, "Issue to Link")

	// Create plan
	plan, err := client.CreatePlan(ws.ID, "Linkable", "")
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	// Link issue
	if err := client.LinkIssuesToPlan(ws.ID, plan.ID, []string{issue.ID}); err != nil {
		t.Fatalf("LinkIssuesToPlan failed: %v", err)
	}

	// Verify link
	retrieved, err := client.GetPlan(ws.ID, plan.ID)
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}
	if len(retrieved.LinkedIssues) != 1 {
		t.Errorf("LinkedIssues length = %d, want 1", len(retrieved.LinkedIssues))
	}

	// Unlink issue
	if err := client.UnlinkIssueFromPlan(ws.ID, plan.ID, issue.ID); err != nil {
		t.Fatalf("UnlinkIssueFromPlan failed: %v", err)
	}

	// Verify unlink
	retrieved, err = client.GetPlan(ws.ID, plan.ID)
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}
	if len(retrieved.LinkedIssues) != 0 {
		t.Errorf("LinkedIssues after unlink length = %d, want 0", len(retrieved.LinkedIssues))
	}
}

func TestClientHealth(t *testing.T) {
	client, cleanup := testClientServer(t)
	defer cleanup()

	if err := client.Health(); err != nil {
		t.Errorf("Health check failed: %v", err)
	}
}

// TestClientSetActor verifies that SetActor does not panic.
// The actor field is unexported; actual behavior is tested through integration tests.
func TestClientSetActor(t *testing.T) {
	c := client.New("http://localhost:8080")
	c.SetActor("test-user") // should not panic
}
