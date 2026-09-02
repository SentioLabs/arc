package main

import (
	"bytes"
	"testing"

	"github.com/sentiolabs/arc/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestFormatReadyIssue_BreadcrumbAndInheritedPriority(t *testing.T) {
	ri := &types.ReadyIssue{
		Issue: types.Issue{
			ID:        "arc-42",
			Status:    types.StatusOpen,
			IssueType: types.TypeTask,
			Priority:  2,
			Title:     "Implement frame parser",
		},
		EffectivePriority: 0,
		Path:              []string{"v1", "Jackery"},
	}

	result := formatReadyIssue(ri)

	assert.Contains(t, result, "[v1 › Jackery]")
	assert.Contains(t, result, "P0*")
}

func TestFormatReadyIssue_StandaloneWithoutInheritedPriority(t *testing.T) {
	ri := &types.ReadyIssue{
		Issue: types.Issue{
			ID:        "arc-7",
			Status:    types.StatusOpen,
			IssueType: types.TypeTask,
			Priority:  1,
			Title:     "Fix flaky test",
		},
		EffectivePriority: 1,
		Path:              nil,
	}

	result := formatReadyIssue(ri)

	assert.Contains(t, result, "[standalone]")
	assert.Contains(t, result, "P1")
	assert.NotContains(t, result, "P1*")
}

func TestRenderRoadmap_MarkersAndCounts(t *testing.T) {
	release := &types.RoadmapNode{
		Issue: types.Issue{
			ID:        "arc-1",
			Title:     "v1",
			Status:    types.StatusOpen,
			IssueType: types.TypeRelease,
			Priority:  0,
		},
		TotalCount:  47,
		ClosedCount: 12,
		Children: []*types.RoadmapNode{
			{
				Issue: types.Issue{
					ID:        "arc-2",
					Title:     "Mobile Foundation",
					Status:    types.StatusClosed,
					IssueType: types.TypeMilestone,
				},
			},
			{
				Issue: types.Issue{
					ID:        "arc-3",
					Title:     "Renogy Monitoring",
					Status:    types.StatusOpen,
					IssueType: types.TypeMilestone,
				},
				TotalCount:  8,
				ClosedCount: 3,
			},
			{
				Issue: types.Issue{
					ID:        "arc-4",
					Title:     "Jackery",
					Status:    types.StatusOpen,
					IssueType: types.TypeMilestone,
				},
				GatedBy: []string{"arc-99"},
			},
			{
				Issue: types.Issue{
					ID:        "arc-5",
					Title:     "ESP32 Shunt",
					Status:    types.StatusOpen,
					IssueType: types.TypeMilestone,
					Labels:    []string{"parallel"},
				},
				TotalCount:  9,
				ClosedCount: 1,
			},
		},
	}

	var buf bytes.Buffer
	renderRoadmap([]*types.RoadmapNode{release}, &buf)
	out := buf.String()

	assert.Contains(t, out, "v1 (release, P0) — 12/47 closed")
	assert.Contains(t, out, "✓ Mobile Foundation (milestone) — closed")
	assert.Contains(t, out, "▶ Renogy Monitoring (milestone) — 3/8 — ACTIVE")
	assert.Contains(t, out, "⏸ Jackery (milestone) — gated by arc-99")
	assert.Contains(t, out, "▶ ESP32 Shunt (milestone, parallel) — 1/9 — ACTIVE")
}
