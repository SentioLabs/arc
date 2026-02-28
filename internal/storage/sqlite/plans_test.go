package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
)

// setupTestStore creates a temporary store for testing.
func setupTestStore(t *testing.T) (*sqlite.Store, func()) {
	t.Helper()

	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cleanup := func() {
		_ = store.Close()
	}

	return store, cleanup
}

// setupTestWorkspace creates a workspace for testing.
func setupTestWorkspace(t *testing.T, store *sqlite.Store) *types.Workspace {
	t.Helper()
	ctx := context.Background()

	ws := &types.Workspace{
		Name:   "Test Workspace",
		Prefix: "test",
	}
	if err := store.CreateWorkspace(ctx, ws); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	return ws
}

// setupTestIssue creates an issue for testing.
func setupTestIssue(t *testing.T, store *sqlite.Store, ws *types.Workspace, title string) *types.Issue {
	t.Helper()
	ctx := context.Background()

	issue := &types.Issue{
		WorkspaceID: ws.ID,
		Title:       title,
		Status:      types.StatusOpen,
		Priority:    2,
		IssueType:   types.TypeTask,
	}
	if err := store.CreateIssue(ctx, issue, "test-actor"); err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	return issue
}

func TestCreateAndGetPlan(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

	plan := &types.Plan{
		ID:          "plan.abc123",
		WorkspaceID: ws.ID,
		Title:       "Test Plan",
		Content:     "Plan content here",
	}

	// Create the plan
	if err := store.CreatePlan(ctx, plan); err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	// Verify timestamps were set
	if plan.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set after CreatePlan")
	}
	if plan.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set after CreatePlan")
	}

	// Get the plan
	retrieved, err := store.GetPlan(ctx, plan.ID)
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}

	if retrieved.ID != plan.ID {
		t.Errorf("ID = %q, want %q", retrieved.ID, plan.ID)
	}
	if retrieved.WorkspaceID != plan.WorkspaceID {
		t.Errorf("WorkspaceID = %q, want %q", retrieved.WorkspaceID, plan.WorkspaceID)
	}
	if retrieved.Title != plan.Title {
		t.Errorf("Title = %q, want %q", retrieved.Title, plan.Title)
	}
	if retrieved.Content != plan.Content {
		t.Errorf("Content = %q, want %q", retrieved.Content, plan.Content)
	}
}

func TestGetPlanNotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	_, err := store.GetPlan(ctx, "plan.nonexistent")
	if err == nil {
		t.Error("GetPlan should return error for nonexistent plan")
	}
}

func TestListPlans(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

	// Create multiple plans
	plans := []*types.Plan{
		{ID: "plan.001", WorkspaceID: ws.ID, Title: "Plan 1", Content: "Content 1"},
		{ID: "plan.002", WorkspaceID: ws.ID, Title: "Plan 2", Content: "Content 2"},
		{ID: "plan.003", WorkspaceID: ws.ID, Title: "Plan 3", Content: "Content 3"},
	}

	for _, p := range plans {
		if err := store.CreatePlan(ctx, p); err != nil {
			t.Fatalf("CreatePlan failed: %v", err)
		}
	}

	// List plans
	listed, err := store.ListPlans(ctx, ws.ID)
	if err != nil {
		t.Fatalf("ListPlans failed: %v", err)
	}

	if len(listed) != len(plans) {
		t.Errorf("ListPlans returned %d plans, want %d", len(listed), len(plans))
	}
}

func TestUpdatePlan(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

	plan := &types.Plan{
		ID:          "plan.update",
		WorkspaceID: ws.ID,
		Title:       "Original Title",
		Content:     "Original Content",
	}
	if err := store.CreatePlan(ctx, plan); err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	// Update the plan
	newTitle := "Updated Title"
	newContent := "Updated Content"
	if err := store.UpdatePlan(ctx, plan.ID, newTitle, newContent); err != nil {
		t.Fatalf("UpdatePlan failed: %v", err)
	}

	// Verify update
	retrieved, err := store.GetPlan(ctx, plan.ID)
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}

	if retrieved.Title != newTitle {
		t.Errorf("Title = %q, want %q", retrieved.Title, newTitle)
	}
	if retrieved.Content != newContent {
		t.Errorf("Content = %q, want %q", retrieved.Content, newContent)
	}
	if !retrieved.UpdatedAt.After(retrieved.CreatedAt) {
		t.Error("UpdatedAt should be after CreatedAt after update")
	}
}

func TestDeletePlan(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

	plan := &types.Plan{
		ID:          "plan.delete",
		WorkspaceID: ws.ID,
		Title:       "To Delete",
		Content:     "Will be deleted",
	}
	if err := store.CreatePlan(ctx, plan); err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	// Delete the plan
	if err := store.DeletePlan(ctx, plan.ID); err != nil {
		t.Fatalf("DeletePlan failed: %v", err)
	}

	// Verify deletion
	_, err := store.GetPlan(ctx, plan.ID)
	if err == nil {
		t.Error("GetPlan should return error after deletion")
	}
}

func TestLinkAndUnlinkIssueToPlan(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

	// Create plan and issues
	plan := &types.Plan{
		ID:          "plan.link",
		WorkspaceID: ws.ID,
		Title:       "Linkable Plan",
		Content:     "Content",
	}
	if err := store.CreatePlan(ctx, plan); err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	issue1 := setupTestIssue(t, store, ws, "Issue 1")
	issue2 := setupTestIssue(t, store, ws, "Issue 2")

	// Link issues to plan
	if err := store.LinkIssueToPlan(ctx, issue1.ID, plan.ID); err != nil {
		t.Fatalf("LinkIssueToPlan failed: %v", err)
	}
	if err := store.LinkIssueToPlan(ctx, issue2.ID, plan.ID); err != nil {
		t.Fatalf("LinkIssueToPlan failed: %v", err)
	}

	// Get linked issues
	linkedIssues, err := store.GetLinkedIssues(ctx, plan.ID)
	if err != nil {
		t.Fatalf("GetLinkedIssues failed: %v", err)
	}
	if len(linkedIssues) != 2 {
		t.Errorf("GetLinkedIssues returned %d issues, want 2", len(linkedIssues))
	}

	// Get linked plans from issue perspective
	linkedPlans, err := store.GetLinkedPlans(ctx, issue1.ID)
	if err != nil {
		t.Fatalf("GetLinkedPlans failed: %v", err)
	}
	if len(linkedPlans) != 1 {
		t.Errorf("GetLinkedPlans returned %d plans, want 1", len(linkedPlans))
	}
	if linkedPlans[0].ID != plan.ID {
		t.Errorf("LinkedPlan ID = %q, want %q", linkedPlans[0].ID, plan.ID)
	}

	// Unlink one issue
	if err := store.UnlinkIssueFromPlan(ctx, issue1.ID, plan.ID); err != nil {
		t.Fatalf("UnlinkIssueFromPlan failed: %v", err)
	}

	// Verify unlink
	linkedIssues, err = store.GetLinkedIssues(ctx, plan.ID)
	if err != nil {
		t.Fatalf("GetLinkedIssues failed: %v", err)
	}
	if len(linkedIssues) != 1 {
		t.Errorf("GetLinkedIssues returned %d issues after unlink, want 1", len(linkedIssues))
	}
}

func TestSetAndGetInlinePlan(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)
	issue := setupTestIssue(t, store, ws, "Issue with Plan")

	// Initially no plan - should return ErrNoPlan
	_, err := store.GetInlinePlan(ctx, issue.ID)
	if !errors.Is(err, sqlite.ErrNoPlan) {
		t.Fatalf("GetInlinePlan should return ErrNoPlan for new issue, got: %v", err)
	}

	// Set inline plan
	planText := "This is the plan:\n1. Step 1\n2. Step 2"
	author := "test-author"
	created, err := store.SetInlinePlan(ctx, issue.ID, author, planText)
	if err != nil {
		t.Fatalf("SetInlinePlan failed: %v", err)
	}

	if created.Text != planText {
		t.Errorf("Created plan Text = %q, want %q", created.Text, planText)
	}
	if created.Author != author {
		t.Errorf("Created plan Author = %q, want %q", created.Author, author)
	}
	if created.CommentType != types.CommentTypePlan {
		t.Errorf("Created plan CommentType = %q, want %q", created.CommentType, types.CommentTypePlan)
	}

	// Get inline plan
	retrieved, err := store.GetInlinePlan(ctx, issue.ID)
	if err != nil {
		t.Fatalf("GetInlinePlan failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetInlinePlan returned nil after SetInlinePlan")
	}
	if retrieved.Text != planText {
		t.Errorf("Retrieved plan Text = %q, want %q", retrieved.Text, planText)
	}
}

func TestPlanHistory(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)
	issue := setupTestIssue(t, store, ws, "Issue with Plan History")

	// Set multiple plan versions
	versions := []string{
		"Plan version 1",
		"Plan version 2 - updated",
		"Plan version 3 - final",
	}

	for _, v := range versions {
		if _, err := store.SetInlinePlan(ctx, issue.ID, "author", v); err != nil {
			t.Fatalf("SetInlinePlan failed: %v", err)
		}
	}

	// Get plan history
	history, err := store.GetPlanHistory(ctx, issue.ID)
	if err != nil {
		t.Fatalf("GetPlanHistory failed: %v", err)
	}

	if len(history) != len(versions) {
		t.Errorf("GetPlanHistory returned %d versions, want %d", len(history), len(versions))
	}

	// History should be in reverse chronological order (newest first)
	// Based on queries, should be ordered by created_at DESC
	if len(history) > 0 && history[0].Text != versions[len(versions)-1] {
		t.Errorf("Latest plan in history = %q, want %q", history[0].Text, versions[len(versions)-1])
	}

	// GetInlinePlan should return the latest
	latest, err := store.GetInlinePlan(ctx, issue.ID)
	if err != nil {
		t.Fatalf("GetInlinePlan failed: %v", err)
	}
	if latest.Text != versions[len(versions)-1] {
		t.Errorf("Latest plan = %q, want %q", latest.Text, versions[len(versions)-1])
	}
}

func TestGetPlanContext(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

	// Create parent (epic) with a plan
	parent := setupTestIssue(t, store, ws, "Parent Epic")
	if _, err := store.SetInlinePlan(ctx, parent.ID, "author", "Parent's master plan"); err != nil {
		t.Fatalf("SetInlinePlan on parent failed: %v", err)
	}

	// Create child issue
	child := setupTestIssue(t, store, ws, "Child Task")

	// Add parent-child dependency
	dep := &types.Dependency{
		IssueID:     child.ID,
		DependsOnID: parent.ID,
		Type:        types.DepParentChild,
	}
	if err := store.AddDependency(ctx, dep, "test-actor"); err != nil {
		t.Fatalf("AddDependency failed: %v", err)
	}

	// Set inline plan on child
	if _, err := store.SetInlinePlan(ctx, child.ID, "author", "Child's specific plan"); err != nil {
		t.Fatalf("SetInlinePlan on child failed: %v", err)
	}

	// Create and link a shared plan
	sharedPlan := &types.Plan{
		ID:          "plan.shared",
		WorkspaceID: ws.ID,
		Title:       "Shared Initiative",
		Content:     "Shared plan content",
	}
	if err := store.CreatePlan(ctx, sharedPlan); err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}
	if err := store.LinkIssueToPlan(ctx, child.ID, sharedPlan.ID); err != nil {
		t.Fatalf("LinkIssueToPlan failed: %v", err)
	}

	// Get plan context for child
	pc, err := store.GetPlanContext(ctx, child.ID)
	if err != nil {
		t.Fatalf("GetPlanContext failed: %v", err)
	}

	// Verify inline plan
	if pc.InlinePlan == nil {
		t.Error("InlinePlan should not be nil")
	} else if pc.InlinePlan.Text != "Child's specific plan" {
		t.Errorf("InlinePlan.Text = %q, want %q", pc.InlinePlan.Text, "Child's specific plan")
	}

	// Verify parent plan
	if pc.ParentPlan == nil {
		t.Error("ParentPlan should not be nil")
	} else if pc.ParentPlan.Text != "Parent's master plan" {
		t.Errorf("ParentPlan.Text = %q, want %q", pc.ParentPlan.Text, "Parent's master plan")
	}
	if pc.ParentIssueID != parent.ID {
		t.Errorf("ParentIssueID = %q, want %q", pc.ParentIssueID, parent.ID)
	}

	// Verify shared plans
	if len(pc.SharedPlans) != 1 {
		t.Errorf("SharedPlans length = %d, want 1", len(pc.SharedPlans))
	} else if pc.SharedPlans[0].ID != sharedPlan.ID {
		t.Errorf("SharedPlans[0].ID = %q, want %q", pc.SharedPlans[0].ID, sharedPlan.ID)
	}

	// Verify HasPlan returns true
	if !pc.HasPlan() {
		t.Error("HasPlan() should return true when plans exist")
	}
}

func TestGetPlanContextEmpty(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)
	issue := setupTestIssue(t, store, ws, "Issue without Plans")

	pc, err := store.GetPlanContext(ctx, issue.ID)
	if err != nil {
		t.Fatalf("GetPlanContext failed: %v", err)
	}

	if pc.InlinePlan != nil {
		t.Error("InlinePlan should be nil for issue without plans")
	}
	if pc.ParentPlan != nil {
		t.Error("ParentPlan should be nil for issue without parent plan")
	}
	if len(pc.SharedPlans) != 0 {
		t.Errorf("SharedPlans length = %d, want 0", len(pc.SharedPlans))
	}
	if pc.HasPlan() {
		t.Error("HasPlan() should return false when no plans exist")
	}
}
