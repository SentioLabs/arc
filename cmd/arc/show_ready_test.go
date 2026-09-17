package main

import (
	"testing"

	"github.com/sentiolabs/arc/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestFormatPlanInfoRetainsPinnedStatus(t *testing.T) {
	plan := &types.Plan{ID: "plan.one", Title: "Architecture", HeadRevision: 9, Lifecycle: "archived"}
	revision := &types.PlanRevisionWithContent{
		PlanRevision: types.PlanRevision{Revision: 2, ReviewStatus: "approved"},
	}
	source := types.GoverningPlanContext{ContainerID: "milestone", ContainerType: types.TypeMilestone}
	result := formatPlanInfo(plan, revision, source)
	for _, want := range []string{"Architecture", "plan.one", "revision 2", "approved", "archived", "milestone"} {
		assert.Contains(t, result, want)
	}
	assert.NotContains(t, result, "revision 9")
}

func TestFormatPendingPlanNotice_WithPending(t *testing.T) {
	result := formatPendingPlanNotice(3)
	assert.Equal(t, "⚠ 3 plan(s) pending review", result)
}

func TestFormatPendingPlanNotice_ZeroPending(t *testing.T) {
	result := formatPendingPlanNotice(0)
	assert.Empty(t, result)
}

func TestFormatPendingPlanNotice_OnePending(t *testing.T) {
	result := formatPendingPlanNotice(1)
	assert.Equal(t, "⚠ 1 plan(s) pending review", result)
}
