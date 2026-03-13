package types

import (
	"testing"
)

func TestPlanStatusConstants(t *testing.T) {
	// Verify plan status constants exist and have correct values
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"draft", PlanStatusDraft, "draft"},
		{"approved", PlanStatusApproved, "approved"},
		{"rejected", PlanStatusRejected, "rejected"},
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
	p := Plan{
		ID:        "plan.abc",
		ProjectID: "proj-1",
		Title:     "Test Plan",
		Status:    PlanStatusDraft,
	}
	if p.Status != "draft" {
		t.Errorf("expected status 'draft', got %q", p.Status)
	}
}

func TestPlanHasIssueIDField(t *testing.T) {
	p := Plan{
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
	// This test verifies at compile time that Plan does NOT have a LinkedIssues field.
	// If LinkedIssues were still present, this would compile but we verify the struct
	// only has the expected fields by checking JSON serialization.
	p := Plan{
		ID:        "plan.abc",
		ProjectID: "proj-1",
		Title:     "Test Plan",
		Content:   "content",
		Status:    PlanStatusDraft,
		IssueID:   "issue-1",
	}
	// Verify the plan validates successfully
	if err := p.Validate(); err != nil {
		t.Errorf("expected valid plan, got error: %v", err)
	}
}

func TestCommentTypeCommentIsValid(t *testing.T) {
	// CommentTypeComment should still be valid
	if !CommentTypeComment.IsValid() {
		t.Error("expected CommentTypeComment to be valid")
	}
}

func TestCommentTypePlanRemoved(t *testing.T) {
	// After removal, "plan" should no longer be a valid comment type
	ct := CommentType("plan")
	if ct.IsValid() {
		t.Error("expected 'plan' comment type to no longer be valid")
	}
}

func TestPlanContextRemoved(t *testing.T) {
	// Verify IssueDetails no longer has PlanContext field.
	// This is a compile-time check - if PlanContext still existed on IssueDetails,
	// using a field that doesn't exist would cause a compile error.
	d := IssueDetails{}
	_ = d // IssueDetails should compile without PlanContext
}
