package sqlite_test

import (
	"context"
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

// setupTestProject creates a project for testing.
func setupTestProject(t *testing.T, store *sqlite.Store) *types.Project {
	t.Helper()
	ctx := context.Background()

	proj := &types.Project{
		Name:   "Test Project",
		Prefix: "test",
	}
	if err := store.CreateProject(ctx, proj); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	return proj
}

// setupTestIssue creates an issue for testing.
func setupTestIssue(t *testing.T, store *sqlite.Store, proj *types.Project, title string) *types.Issue {
	t.Helper()
	ctx := context.Background()

	issue := &types.Issue{
		ProjectID: proj.ID,
		Title:     title,
		Status:    types.StatusOpen,
		Priority:  2,
		IssueType: types.TypeTask,
	}
	if err := store.CreateIssue(ctx, issue, "test-actor"); err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	return issue
}

func TestCreateOrUpdatePlan_Create(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)
	issue := setupTestIssue(t, store, proj, "Issue with plan")

	plan := &types.Plan{
		ProjectID: proj.ID,
		IssueID:   issue.ID,
		Title:     "Test Plan",
		Content:   "Plan content here",
		Status:    types.PlanStatusDraft,
	}

	err := store.CreateOrUpdatePlan(ctx, plan)
	if err != nil {
		t.Fatalf("CreateOrUpdatePlan failed: %v", err)
	}

	// ID should be generated
	if plan.ID == "" {
		t.Error("expected plan ID to be generated")
	}

	// Timestamps should be set
	if plan.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if plan.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}

	// Status should be returned
	if plan.Status != types.PlanStatusDraft {
		t.Errorf("Status = %q, want %q", plan.Status, types.PlanStatusDraft)
	}

	// Verify we can retrieve it
	got, err := store.GetPlan(ctx, plan.ID)
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}
	if got.Title != "Test Plan" {
		t.Errorf("Title = %q, want %q", got.Title, "Test Plan")
	}
	if got.IssueID != issue.ID {
		t.Errorf("IssueID = %q, want %q", got.IssueID, issue.ID)
	}
}

func TestCreateOrUpdatePlan_Upsert(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)
	issue := setupTestIssue(t, store, proj, "Issue for upsert")

	// Create initial plan
	plan := &types.Plan{
		ProjectID: proj.ID,
		IssueID:   issue.ID,
		Title:     "Original Title",
		Content:   "Original Content",
		Status:    types.PlanStatusDraft,
	}
	if err := store.CreateOrUpdatePlan(ctx, plan); err != nil {
		t.Fatalf("first CreateOrUpdatePlan failed: %v", err)
	}
	originalID := plan.ID

	// Update plan for same issue
	plan2 := &types.Plan{
		ProjectID: proj.ID,
		IssueID:   issue.ID,
		Title:     "Updated Title",
		Content:   "Updated Content",
		Status:    types.PlanStatusDraft,
	}
	if err := store.CreateOrUpdatePlan(ctx, plan2); err != nil {
		t.Fatalf("second CreateOrUpdatePlan failed: %v", err)
	}

	// Should keep the same ID (upsert on issue_id)
	if plan2.ID != originalID {
		t.Errorf("upsert should keep original ID %q, got %q", originalID, plan2.ID)
	}

	// Verify content was updated
	got, err := store.GetPlan(ctx, originalID)
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}
	if got.Title != "Updated Title" {
		t.Errorf("Title = %q, want %q", got.Title, "Updated Title")
	}
	if got.Content != "Updated Content" {
		t.Errorf("Content = %q, want %q", got.Content, "Updated Content")
	}

	// Verify no duplicates
	plans, err := store.ListPlans(ctx, proj.ID, "")
	if err != nil {
		t.Fatalf("ListPlans failed: %v", err)
	}
	if len(plans) != 1 {
		t.Errorf("expected 1 plan after upsert, got %d", len(plans))
	}
}

func TestGetPlanByIssueID(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)
	issue := setupTestIssue(t, store, proj, "Issue with plan")

	// No plan yet — should return an error
	got, err := store.GetPlanByIssueID(ctx, issue.ID)
	if err == nil {
		t.Fatal("GetPlanByIssueID should return error for missing plan")
	}
	if got != nil {
		t.Error("GetPlanByIssueID should return nil plan when not found")
	}

	// Create a plan
	plan := &types.Plan{
		ProjectID: proj.ID,
		IssueID:   issue.ID,
		Title:     "Test Plan",
		Content:   "Content",
		Status:    types.PlanStatusDraft,
	}
	if err := store.CreateOrUpdatePlan(ctx, plan); err != nil {
		t.Fatalf("CreateOrUpdatePlan failed: %v", err)
	}

	// Now should find it
	got, err = store.GetPlanByIssueID(ctx, issue.ID)
	if err != nil {
		t.Fatalf("GetPlanByIssueID failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetPlanByIssueID returned nil after creating plan")
	}
	if got.Title != "Test Plan" {
		t.Errorf("Title = %q, want %q", got.Title, "Test Plan")
	}
	if got.IssueID != issue.ID {
		t.Errorf("IssueID = %q, want %q", got.IssueID, issue.ID)
	}
}

func TestGetPlan(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)
	issue := setupTestIssue(t, store, proj, "Issue for GetPlan")

	plan := &types.Plan{
		ProjectID: proj.ID,
		IssueID:   issue.ID,
		Title:     "Get Plan Test",
		Content:   "Content for get",
		Status:    types.PlanStatusDraft,
	}
	if err := store.CreateOrUpdatePlan(ctx, plan); err != nil {
		t.Fatalf("CreateOrUpdatePlan failed: %v", err)
	}

	got, err := store.GetPlan(ctx, plan.ID)
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}

	if got.ID != plan.ID {
		t.Errorf("ID = %q, want %q", got.ID, plan.ID)
	}
	if got.ProjectID != proj.ID {
		t.Errorf("ProjectID = %q, want %q", got.ProjectID, proj.ID)
	}
	if got.Title != "Get Plan Test" {
		t.Errorf("Title = %q, want %q", got.Title, "Get Plan Test")
	}
	if got.Content != "Content for get" {
		t.Errorf("Content = %q, want %q", got.Content, "Content for get")
	}
	if got.Status != types.PlanStatusDraft {
		t.Errorf("Status = %q, want %q", got.Status, types.PlanStatusDraft)
	}

	// Not found case
	_, err = store.GetPlan(ctx, "plan.nonexistent")
	if err == nil {
		t.Error("GetPlan should return error for nonexistent plan")
	}
}

func TestListPlans(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	issue1 := setupTestIssue(t, store, proj, "Issue 1")
	issue2 := setupTestIssue(t, store, proj, "Issue 2")
	issue3 := setupTestIssue(t, store, proj, "Issue 3")

	// Create plans with different statuses
	plans := []*types.Plan{
		{ProjectID: proj.ID, IssueID: issue1.ID, Title: "Plan 1", Content: "C1", Status: types.PlanStatusDraft},
		{ProjectID: proj.ID, IssueID: issue2.ID, Title: "Plan 2", Content: "C2", Status: types.PlanStatusApproved},
		{ProjectID: proj.ID, IssueID: issue3.ID, Title: "Plan 3", Content: "C3", Status: types.PlanStatusDraft},
	}
	for _, p := range plans {
		if err := store.CreateOrUpdatePlan(ctx, p); err != nil {
			t.Fatalf("CreateOrUpdatePlan failed: %v", err)
		}
	}

	// List all plans (no status filter)
	all, err := store.ListPlans(ctx, proj.ID, "")
	if err != nil {
		t.Fatalf("ListPlans failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("ListPlans (no filter) returned %d plans, want 3", len(all))
	}

	// List with status filter
	drafts, err := store.ListPlans(ctx, proj.ID, types.PlanStatusDraft)
	if err != nil {
		t.Fatalf("ListPlans (draft) failed: %v", err)
	}
	if len(drafts) != 2 {
		t.Errorf("ListPlans (draft) returned %d plans, want 2", len(drafts))
	}

	approved, err := store.ListPlans(ctx, proj.ID, types.PlanStatusApproved)
	if err != nil {
		t.Fatalf("ListPlans (approved) failed: %v", err)
	}
	if len(approved) != 1 {
		t.Errorf("ListPlans (approved) returned %d plans, want 1", len(approved))
	}
}

func TestUpdatePlanStatus(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)
	issue := setupTestIssue(t, store, proj, "Issue for status update")

	plan := &types.Plan{
		ProjectID: proj.ID,
		IssueID:   issue.ID,
		Title:     "Status Plan",
		Content:   "Content",
		Status:    types.PlanStatusDraft,
	}
	if err := store.CreateOrUpdatePlan(ctx, plan); err != nil {
		t.Fatalf("CreateOrUpdatePlan failed: %v", err)
	}

	// Update status
	if err := store.UpdatePlanStatus(ctx, plan.ID, types.PlanStatusApproved); err != nil {
		t.Fatalf("UpdatePlanStatus failed: %v", err)
	}

	// Verify
	got, err := store.GetPlan(ctx, plan.ID)
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}
	if got.Status != types.PlanStatusApproved {
		t.Errorf("Status = %q, want %q", got.Status, types.PlanStatusApproved)
	}
	if !got.UpdatedAt.After(got.CreatedAt) {
		t.Error("UpdatedAt should be after CreatedAt after status update")
	}
}

func TestUpdatePlanContent(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)
	issue := setupTestIssue(t, store, proj, "Issue for content update")

	plan := &types.Plan{
		ProjectID: proj.ID,
		IssueID:   issue.ID,
		Title:     "Original Title",
		Content:   "Original Content",
		Status:    types.PlanStatusDraft,
	}
	if err := store.CreateOrUpdatePlan(ctx, plan); err != nil {
		t.Fatalf("CreateOrUpdatePlan failed: %v", err)
	}

	// Update content
	if err := store.UpdatePlanContent(ctx, plan.ID, "New Title", "New Content"); err != nil {
		t.Fatalf("UpdatePlanContent failed: %v", err)
	}

	// Verify
	got, err := store.GetPlan(ctx, plan.ID)
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}
	if got.Title != "New Title" {
		t.Errorf("Title = %q, want %q", got.Title, "New Title")
	}
	if got.Content != "New Content" {
		t.Errorf("Content = %q, want %q", got.Content, "New Content")
	}
	if !got.UpdatedAt.After(got.CreatedAt) {
		t.Error("UpdatedAt should be after CreatedAt after content update")
	}
}

func TestDeletePlan(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)
	issue := setupTestIssue(t, store, proj, "Issue for delete")

	plan := &types.Plan{
		ProjectID: proj.ID,
		IssueID:   issue.ID,
		Title:     "To Delete",
		Content:   "Will be deleted",
		Status:    types.PlanStatusDraft,
	}
	if err := store.CreateOrUpdatePlan(ctx, plan); err != nil {
		t.Fatalf("CreateOrUpdatePlan failed: %v", err)
	}

	// Delete
	if err := store.DeletePlan(ctx, plan.ID); err != nil {
		t.Fatalf("DeletePlan failed: %v", err)
	}

	// Verify deletion
	_, err := store.GetPlan(ctx, plan.ID)
	if err == nil {
		t.Error("GetPlan should return error after deletion")
	}
}

func TestCountPlansByStatus(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	issue1 := setupTestIssue(t, store, proj, "Count Issue 1")
	issue2 := setupTestIssue(t, store, proj, "Count Issue 2")
	issue3 := setupTestIssue(t, store, proj, "Count Issue 3")

	plans := []*types.Plan{
		{ProjectID: proj.ID, IssueID: issue1.ID, Title: "Plan 1", Content: "C1", Status: types.PlanStatusDraft},
		{ProjectID: proj.ID, IssueID: issue2.ID, Title: "Plan 2", Content: "C2", Status: types.PlanStatusDraft},
		{ProjectID: proj.ID, IssueID: issue3.ID, Title: "Plan 3", Content: "C3", Status: types.PlanStatusApproved},
	}
	for _, p := range plans {
		if err := store.CreateOrUpdatePlan(ctx, p); err != nil {
			t.Fatalf("CreateOrUpdatePlan failed: %v", err)
		}
	}

	draftCount, err := store.CountPlansByStatus(ctx, proj.ID, types.PlanStatusDraft)
	if err != nil {
		t.Fatalf("CountPlansByStatus (draft) failed: %v", err)
	}
	if draftCount != 2 {
		t.Errorf("draft count = %d, want 2", draftCount)
	}

	approvedCount, err := store.CountPlansByStatus(ctx, proj.ID, types.PlanStatusApproved)
	if err != nil {
		t.Fatalf("CountPlansByStatus (approved) failed: %v", err)
	}
	if approvedCount != 1 {
		t.Errorf("approved count = %d, want 1", approvedCount)
	}

	rejectedCount, err := store.CountPlansByStatus(ctx, proj.ID, types.PlanStatusRejected)
	if err != nil {
		t.Fatalf("CountPlansByStatus (rejected) failed: %v", err)
	}
	if rejectedCount != 0 {
		t.Errorf("rejected count = %d, want 0", rejectedCount)
	}
}
