package types //nolint:testpackage // contract assertions pin the package's own declarations

import (
	"testing"
	"time"
)

// --- Contract assertions ---
// These verify the approved design contracts for the planner unified-review
// feature. Do NOT modify without updating the approved plan.

//nolint:staticcheck // QF1011: the explicit types are the contract being asserted
func TestPlanCommentContract(t *testing.T) {
	pc := PlanComment{}
	var _ *int = pc.LineNumber
	var _ *PlanCommentAnchor = pc.Anchor
	var _ *time.Time = pc.UpdatedAt
	var _ *time.Time = pc.ResolvedAt

	a := PlanCommentAnchor{}
	var _ int = a.LineStart
	var _ int = a.LineEnd
	var _ string = a.QuotedText
	var _ int = a.Occurrence
	var _ string = a.HeadingSlug
	var _ string = a.ContextBefore
	var _ string = a.ContextAfter

	if PlanStatusChangesRequested != "changes_requested" {
		t.Fatalf("PlanStatusChangesRequested = %q", PlanStatusChangesRequested)
	}
}
