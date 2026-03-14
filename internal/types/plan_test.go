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
		{"in_review", types.PlanStatusInReview, "in_review"},
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

func TestPlanHasFilePathField(t *testing.T) {
	p := types.Plan{
		ID:       "plan.abc",
		FilePath: "/tmp/plan.md",
		Status:   types.PlanStatusDraft,
	}
	if p.FilePath != "/tmp/plan.md" {
		t.Errorf("expected file_path '/tmp/plan.md', got %q", p.FilePath)
	}
}

func TestPlanCommentLineNumber(t *testing.T) {
	line := 42
	c := types.PlanComment{
		ID:         "comment-1",
		PlanID:     "plan.abc",
		LineNumber: &line,
		Content:    "Looks good",
	}
	if c.LineNumber == nil || *c.LineNumber != 42 {
		t.Errorf("expected line_number 42, got %v", c.LineNumber)
	}
}

func TestPlanWithContent(t *testing.T) {
	pwc := types.PlanWithContent{
		Plan: types.Plan{
			ID:       "plan.abc",
			FilePath: "/tmp/plan.md",
			Status:   types.PlanStatusDraft,
		},
		Content: "# My Plan",
	}
	if pwc.Content != "# My Plan" {
		t.Errorf("expected content '# My Plan', got %q", pwc.Content)
	}
	if pwc.FilePath != "/tmp/plan.md" {
		t.Errorf("expected embedded file_path '/tmp/plan.md', got %q", pwc.FilePath)
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
