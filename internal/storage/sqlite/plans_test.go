package sqlite_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

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

func TestCreatePlan(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	plan := &types.Plan{
		ID:       "plan.abc123",
		FilePath: "/tmp/plans/my-plan.md",
		Status:   types.PlanStatusDraft,
	}

	err := store.CreatePlan(ctx, plan)
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	// Timestamps should be set
	if plan.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if plan.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}

	// Verify we can retrieve it
	got, err := store.GetPlan(ctx, plan.ID)
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}
	if got.ID != plan.ID {
		t.Errorf("ID = %q, want %q", got.ID, plan.ID)
	}
	if got.FilePath != "/tmp/plans/my-plan.md" {
		t.Errorf("FilePath = %q, want %q", got.FilePath, "/tmp/plans/my-plan.md")
	}
	if got.Status != types.PlanStatusDraft {
		t.Errorf("Status = %q, want %q", got.Status, types.PlanStatusDraft)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set on retrieved plan")
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set on retrieved plan")
	}
}

func TestGetPlan_NotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	_, err := store.GetPlan(ctx, "plan.nonexistent")
	if err == nil {
		t.Error("GetPlan should return error for nonexistent plan")
	}
}

func TestUpdatePlanStatus(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	plan := &types.Plan{
		ID:       "plan.status1",
		FilePath: "/tmp/plans/status-test.md",
		Status:   types.PlanStatusDraft,
	}
	if err := store.CreatePlan(ctx, plan); err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	// Update to in_review
	if err := store.UpdatePlanStatus(ctx, plan.ID, types.PlanStatusInReview); err != nil {
		t.Fatalf("UpdatePlanStatus to in_review failed: %v", err)
	}

	got, err := store.GetPlan(ctx, plan.ID)
	if err != nil {
		t.Fatalf("GetPlan after in_review failed: %v", err)
	}
	if got.Status != types.PlanStatusInReview {
		t.Errorf("Status = %q, want %q", got.Status, types.PlanStatusInReview)
	}
	if !got.UpdatedAt.After(got.CreatedAt) {
		t.Error("UpdatedAt should be after CreatedAt after status update")
	}

	// Update to approved
	if err := store.UpdatePlanStatus(ctx, plan.ID, types.PlanStatusApproved); err != nil {
		t.Fatalf("UpdatePlanStatus to approved failed: %v", err)
	}

	got, err = store.GetPlan(ctx, plan.ID)
	if err != nil {
		t.Fatalf("GetPlan after approved failed: %v", err)
	}
	if got.Status != types.PlanStatusApproved {
		t.Errorf("Status = %q, want %q", got.Status, types.PlanStatusApproved)
	}
}

func TestDeletePlan(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	plan := &types.Plan{
		ID:       "plan.todelete",
		FilePath: "/tmp/plans/delete-me.md",
		Status:   types.PlanStatusDraft,
	}
	if err := store.CreatePlan(ctx, plan); err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
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

func TestCreatePlanComment(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	plan := &types.Plan{
		ID:       "plan.comments1",
		FilePath: "/tmp/plans/commented.md",
		Status:   types.PlanStatusDraft,
	}
	if err := store.CreatePlan(ctx, plan); err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	// Add a line-level comment (line_number=42)
	line42 := 42
	comment1 := &types.PlanComment{
		ID:         "pc.line42",
		PlanID:     plan.ID,
		LineNumber: &line42,
		Content:    "This line needs rework",
	}
	if err := store.CreatePlanComment(ctx, comment1); err != nil {
		t.Fatalf("CreatePlanComment (line-level) failed: %v", err)
	}
	if comment1.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set on line-level comment")
	}

	// Add an overall comment (nil line_number)
	comment2 := &types.PlanComment{
		ID:      "pc.overall",
		PlanID:  plan.ID,
		Content: "Overall looks good",
	}
	if err := store.CreatePlanComment(ctx, comment2); err != nil {
		t.Fatalf("CreatePlanComment (overall) failed: %v", err)
	}

	// List comments
	comments, err := store.ListPlanComments(ctx, plan.ID)
	if err != nil {
		t.Fatalf("ListPlanComments failed: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}

	// Verify first comment (line-level)
	if comments[0].ID != "pc.line42" {
		t.Errorf("first comment ID = %q, want %q", comments[0].ID, "pc.line42")
	}
	if comments[0].LineNumber == nil || *comments[0].LineNumber != 42 {
		t.Errorf("first comment LineNumber = %v, want 42", comments[0].LineNumber)
	}
	if comments[0].Content != "This line needs rework" {
		t.Errorf("first comment Content = %q, want %q", comments[0].Content, "This line needs rework")
	}

	// Verify second comment (overall, nil line_number)
	if comments[1].ID != "pc.overall" {
		t.Errorf("second comment ID = %q, want %q", comments[1].ID, "pc.overall")
	}
	if comments[1].LineNumber != nil {
		t.Errorf("second comment LineNumber = %v, want nil", comments[1].LineNumber)
	}
	if comments[1].Content != "Overall looks good" {
		t.Errorf("second comment Content = %q, want %q", comments[1].Content, "Overall looks good")
	}
}

func TestListPlanComments_Empty(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	plan := &types.Plan{
		ID:       "plan.nocomments",
		FilePath: "/tmp/plans/empty-comments.md",
		Status:   types.PlanStatusDraft,
	}
	if err := store.CreatePlan(ctx, plan); err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	comments, err := store.ListPlanComments(ctx, plan.ID)
	if err != nil {
		t.Fatalf("ListPlanComments failed: %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("expected 0 comments, got %d", len(comments))
	}
}

func TestDeletePlan_CascadesComments(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	plan := &types.Plan{
		ID:       "plan.cascade1",
		FilePath: "/tmp/plans/cascade-test.md",
		Status:   types.PlanStatusDraft,
	}
	if err := store.CreatePlan(ctx, plan); err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	// Add comments
	line10 := 10
	comment1 := &types.PlanComment{
		ID:         "pc.cascade1",
		PlanID:     plan.ID,
		LineNumber: &line10,
		Content:    "Comment 1",
	}
	if err := store.CreatePlanComment(ctx, comment1); err != nil {
		t.Fatalf("CreatePlanComment 1 failed: %v", err)
	}

	comment2 := &types.PlanComment{
		ID:      "pc.cascade2",
		PlanID:  plan.ID,
		Content: "Comment 2",
	}
	if err := store.CreatePlanComment(ctx, comment2); err != nil {
		t.Fatalf("CreatePlanComment 2 failed: %v", err)
	}

	// Verify comments exist
	comments, err := store.ListPlanComments(ctx, plan.ID)
	if err != nil {
		t.Fatalf("ListPlanComments before delete failed: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments before delete, got %d", len(comments))
	}

	// Delete plan
	if err := store.DeletePlan(ctx, plan.ID); err != nil {
		t.Fatalf("DeletePlan failed: %v", err)
	}

	// Verify comments are gone (cascade)
	comments, err = store.ListPlanComments(ctx, plan.ID)
	if err != nil {
		t.Fatalf("ListPlanComments after delete failed: %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("expected 0 comments after plan deletion (cascade), got %d", len(comments))
	}
}

func TestPlanCommentAnchorRoundTrip(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	plan := &types.Plan{
		ID:       "plan.anchor1",
		FilePath: "/tmp/plans/anchor.md",
		Status:   types.PlanStatusDraft,
	}
	if err := store.CreatePlan(ctx, plan); err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	anchor := &types.PlanCommentAnchor{
		LineStart:     12,
		LineEnd:       15,
		QuotedText:    "the quick fox",
		Occurrence:    1,
		HeadingSlug:   "data-model",
		ContextBefore: "before ",
		ContextAfter:  " after",
	}
	comment := &types.PlanComment{
		ID:      "pc.anchor1",
		PlanID:  plan.ID,
		Content: "Consider rewording this",
		Anchor:  anchor,
	}
	if err := store.CreatePlanComment(ctx, comment); err != nil {
		t.Fatalf("CreatePlanComment (anchored) failed: %v", err)
	}

	comments, err := store.ListPlanComments(ctx, plan.ID)
	if err != nil {
		t.Fatalf("ListPlanComments failed: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}

	got := comments[0]
	if got.Anchor == nil {
		t.Fatal("expected Anchor to be non-nil")
	}
	if !reflect.DeepEqual(got.Anchor, anchor) {
		t.Errorf("Anchor = %+v, want %+v", got.Anchor, anchor)
	}
}

func TestPlanCommentAnchorNil(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	plan := &types.Plan{
		ID:       "plan.noanchor1",
		FilePath: "/tmp/plans/no-anchor.md",
		Status:   types.PlanStatusDraft,
	}
	if err := store.CreatePlan(ctx, plan); err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	comment := &types.PlanComment{
		ID:      "pc.noanchor1",
		PlanID:  plan.ID,
		Content: "Overall comment, no anchor",
	}
	if err := store.CreatePlanComment(ctx, comment); err != nil {
		t.Fatalf("CreatePlanComment (no anchor) failed: %v", err)
	}

	comments, err := store.ListPlanComments(ctx, plan.ID)
	if err != nil {
		t.Fatalf("ListPlanComments failed: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}

	got := comments[0]
	if got.Anchor != nil {
		t.Errorf("Anchor = %+v, want nil", got.Anchor)
	}
	if got.UpdatedAt != nil {
		t.Errorf("UpdatedAt = %v, want nil", got.UpdatedAt)
	}
	if got.ResolvedAt != nil {
		t.Errorf("ResolvedAt = %v, want nil", got.ResolvedAt)
	}
}

func TestUpdatePlanComment(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	plan := &types.Plan{
		ID:       "plan.update1",
		FilePath: "/tmp/plans/update.md",
		Status:   types.PlanStatusDraft,
	}
	if err := store.CreatePlan(ctx, plan); err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	anchor := &types.PlanCommentAnchor{
		LineStart:  1,
		LineEnd:    2,
		QuotedText: "original text",
		Occurrence: 0,
	}
	comment := &types.PlanComment{
		ID:      "pc.update1",
		PlanID:  plan.ID,
		Content: "original content",
		Anchor:  anchor,
	}
	if err := store.CreatePlanComment(ctx, comment); err != nil {
		t.Fatalf("CreatePlanComment failed: %v", err)
	}

	updatedAt := time.Now().UTC().Truncate(time.Second)
	resolvedAt := updatedAt.Add(time.Minute)

	comment.Content = "updated content"
	comment.Anchor.QuotedText = "updated quoted text"
	comment.UpdatedAt = &updatedAt
	comment.ResolvedAt = &resolvedAt

	if err := store.UpdatePlanComment(ctx, comment); err != nil {
		t.Fatalf("UpdatePlanComment failed: %v", err)
	}

	got, err := store.GetPlanComment(ctx, comment.ID)
	if err != nil {
		t.Fatalf("GetPlanComment failed: %v", err)
	}
	if got.Content != "updated content" {
		t.Errorf("Content = %q, want %q", got.Content, "updated content")
	}
	if got.Anchor == nil || got.Anchor.QuotedText != "updated quoted text" {
		t.Errorf("Anchor.QuotedText = %+v, want %q", got.Anchor, "updated quoted text")
	}
	if got.Anchor.LineStart != 1 || got.Anchor.LineEnd != 2 {
		t.Errorf("Anchor line range = %d-%d, want 1-2", got.Anchor.LineStart, got.Anchor.LineEnd)
	}
	if got.UpdatedAt == nil || !got.UpdatedAt.Equal(updatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt, updatedAt)
	}
	if got.ResolvedAt == nil || !got.ResolvedAt.Equal(resolvedAt) {
		t.Errorf("ResolvedAt = %v, want %v", got.ResolvedAt, resolvedAt)
	}

	// Clear ResolvedAt back to nil on a second update.
	comment.ResolvedAt = nil
	if err := store.UpdatePlanComment(ctx, comment); err != nil {
		t.Fatalf("UpdatePlanComment (clear ResolvedAt) failed: %v", err)
	}

	got, err = store.GetPlanComment(ctx, comment.ID)
	if err != nil {
		t.Fatalf("GetPlanComment failed: %v", err)
	}
	if got.ResolvedAt != nil {
		t.Errorf("ResolvedAt = %v, want nil after clearing", got.ResolvedAt)
	}
	if got.UpdatedAt == nil || !got.UpdatedAt.Equal(updatedAt) {
		t.Errorf("UpdatedAt = %v, want %v (unchanged)", got.UpdatedAt, updatedAt)
	}
}

func TestDeletePlanComment(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	plan := &types.Plan{
		ID:       "plan.deletecomment1",
		FilePath: "/tmp/plans/delete-comment.md",
		Status:   types.PlanStatusDraft,
	}
	if err := store.CreatePlan(ctx, plan); err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	comment1 := &types.PlanComment{
		ID:      "pc.keep1",
		PlanID:  plan.ID,
		Content: "keep me",
	}
	comment2 := &types.PlanComment{
		ID:      "pc.delete1",
		PlanID:  plan.ID,
		Content: "delete me",
	}
	if err := store.CreatePlanComment(ctx, comment1); err != nil {
		t.Fatalf("CreatePlanComment 1 failed: %v", err)
	}
	if err := store.CreatePlanComment(ctx, comment2); err != nil {
		t.Fatalf("CreatePlanComment 2 failed: %v", err)
	}

	if err := store.DeletePlanComment(ctx, comment2.ID); err != nil {
		t.Fatalf("DeletePlanComment failed: %v", err)
	}

	comments, err := store.ListPlanComments(ctx, plan.ID)
	if err != nil {
		t.Fatalf("ListPlanComments failed: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment after delete, got %d", len(comments))
	}
	if comments[0].ID != comment1.ID {
		t.Errorf("remaining comment ID = %q, want %q", comments[0].ID, comment1.ID)
	}

	if _, err := store.GetPlanComment(ctx, comment2.ID); err == nil {
		t.Error("GetPlanComment should return error for deleted comment")
	}
}
