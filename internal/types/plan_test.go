package types_test

import (
	"testing"

	"github.com/sentiolabs/arc/internal/types"
)

func TestPlanStatusConstants(t *testing.T) {
	// Verify plan status constants exist and have correct values
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"draft", types.PlanStatusDraft, "draft"},
		{"approved", types.PlanStatusApproved, "approved"},
		{"rejected", types.PlanStatusRejected, "rejected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.constant)
			}
		})
	}
}

func TestPlanHasStatusField(t *testing.T) {
	p := types.Plan{
		ID:        "plan.abc",
		ProjectID: "proj-1",
		Title:     "Test Plan",
		Status:    types.PlanStatusDraft,
	}
	if p.Status != "draft" {
		t.Errorf("expected status 'draft', got %q", p.Status)
	}
}

func TestPlanHasIssueIDField(t *testing.T) {
	p := types.Plan{
		ID:        "plan.abc",
		ProjectID: "proj-1",
		Title:     "Test Plan",
		IssueID:   "issue-123",
	}
	if p.IssueID != "issue-123" {
		t.Errorf("expected issue_id 'issue-123', got %q", p.IssueID)
	}
}

func TestPlanNoLinkedIssuesField(t *testing.T) {
	p := types.Plan{
		ID:        "plan.abc",
		ProjectID: "proj-1",
		Title:     "Test Plan",
		Content:   "content",
		Status:    types.PlanStatusDraft,
		IssueID:   "issue-1",
	}
	if err := p.Validate(); err != nil {
		t.Errorf("expected valid plan, got error: %v", err)
	}
}

func TestCommentTypeCommentIsValid(t *testing.T) {
	if !types.CommentTypeComment.IsValid() {
		t.Error("expected CommentTypeComment to be valid")
	}
}

func TestCommentTypePlanRemoved(t *testing.T) {
	ct := types.CommentType("plan")
	if ct.IsValid() {
		t.Error("expected 'plan' comment type to no longer be valid")
	}
}

func TestPlanContextRemoved(t *testing.T) {
	d := types.IssueDetails{}
	_ = d // IssueDetails should compile without PlanContext
}
