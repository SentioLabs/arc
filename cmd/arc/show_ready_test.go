package main

import (
	"testing"

	"github.com/sentiolabs/arc/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestFormatPlanInfo_DraftStatus(t *testing.T) {
	plan := &types.Plan{
		Title:  "Implement feature X",
		Status: "draft",
	}

	result := formatPlanInfo(plan)

	assert.Contains(t, result, "Plan [draft]:")
	assert.Contains(t, result, "  Implement feature X")
	assert.Contains(t, result, "  (pending review)")
}

func TestFormatPlanInfo_ApprovedStatus(t *testing.T) {
	plan := &types.Plan{
		Title:  "Implement feature X",
		Status: "approved",
	}

	result := formatPlanInfo(plan)

	assert.Contains(t, result, "Plan [approved]:")
	assert.Contains(t, result, "  Implement feature X")
	assert.NotContains(t, result, "(pending review)")
}

func TestFormatPlanInfo_NilPlan(t *testing.T) {
	result := formatPlanInfo(nil)
	assert.Empty(t, result)
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
