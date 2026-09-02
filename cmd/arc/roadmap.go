// Roadmap read surfaces: breadcrumb and inherited-priority markers for
// `arc ready` rows, and the `arc roadmap` tree command that renders the
// release/milestone hierarchy with progress counts and gating info.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sentiolabs/arc/internal/types"
	"github.com/spf13/cobra"
)

// roadmapCmd shows the release/milestone tree with progress and gating.
var roadmapCmd = &cobra.Command{
	Use:   "roadmap",
	Short: "Show the release/milestone tree with progress and gating",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getProjectID()
		if err != nil {
			return err
		}

		nodes, err := c.GetRoadmap(wsID)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(nodes)
			return nil
		}

		if len(nodes) == 0 {
			fmt.Println(`No releases or milestones. Create one: arc create "v1" --type=release`)
			return nil
		}

		renderRoadmap(nodes, os.Stdout)
		return nil
	},
}

// formatReadyIssue renders one ready row with roadmap context:
//
//	○ arc-42 [P0*] [task] [v1 › Jackery] - Implement frame parser
//
// The * marks a priority inherited from an ancestor rather than the issue's
// own priority.
func formatReadyIssue(ri *types.ReadyIssue) string {
	icon := statusIconOpen
	switch ri.Status {
	case types.StatusInProgress:
		icon = "◐" // ◐
	case types.StatusBlocked:
		icon = "◌" // ◌
	case types.StatusClosed:
		icon = statusIconClosed
	case types.StatusDeferred:
		icon = "◇" // ◇
	}

	crumb := "[standalone]"
	if len(ri.Path) > 0 {
		crumb = "[" + strings.Join(ri.Path, " › ") + "]"
	}

	prio := fmt.Sprintf("P%d", ri.EffectivePriority)
	if ri.EffectivePriority != ri.Priority {
		prio += "*"
	}

	labelStr := ""
	if len(ri.Labels) > 0 {
		labelStr = " [" + strings.Join(ri.Labels, " ") + "]"
	}

	return fmt.Sprintf("%s %s [%s] [%s]%s %s - %s",
		icon, ri.ID, prio, string(ri.IssueType), labelStr, crumb, ri.Title)
}

// renderRoadmap writes the roadmap tree to w: each root node (release or
// standalone container) on its own line with progress counts, followed by
// its descendants as a box-drawing tree with marker and gating info.
func renderRoadmap(nodes []*types.RoadmapNode, w io.Writer) {
	for _, node := range nodes {
		_, _ = fmt.Fprintln(w, roadmapRootLine(node))
		renderRoadmapChildren(node.Children, w, "")
	}
}

// renderRoadmapChildren recurses through a node's children, drawing the
// standard box-drawing tree (├──/└── branches, │ /blank continuation).
func renderRoadmapChildren(children []*types.RoadmapNode, w io.Writer, prefix string) {
	for i, child := range children {
		last := i == len(children)-1
		branch, nextPrefix := "├── ", prefix+"│   "
		if last {
			branch, nextPrefix = "└── ", prefix+"    "
		}
		_, _ = fmt.Fprintln(w, prefix+branch+roadmapChildLine(child))
		renderRoadmapChildren(child.Children, w, nextPrefix)
	}
}

// roadmapRootLine renders a top-level container's summary line, e.g.
// "v1 (release, P0) — 12/47 closed".
func roadmapRootLine(node *types.RoadmapNode) string {
	return fmt.Sprintf("%s (%s, P%d) — %d/%d closed",
		node.Issue.Title, roadmapTypeLabel(node), node.Issue.Priority, node.ClosedCount, node.TotalCount)
}

// roadmapChildLine renders one non-root container line with its status
// marker: ✓ for closed, ⏸ for gated or deferred/blocked, ▶ + ACTIVE otherwise.
func roadmapChildLine(node *types.RoadmapNode) string {
	typeLabel := roadmapTypeLabel(node)
	switch {
	case node.Issue.Status == types.StatusClosed:
		return fmt.Sprintf("✓ %s (%s) — closed", node.Issue.Title, typeLabel)
	case len(node.GatedBy) > 0:
		return fmt.Sprintf("⏸ %s (%s) — gated by %s", node.Issue.Title, typeLabel, strings.Join(node.GatedBy, ", "))
	case node.Issue.Status == types.StatusDeferred || node.Issue.Status == types.StatusBlocked:
		return fmt.Sprintf("⏸ %s (%s) — %s", node.Issue.Title, typeLabel, node.Issue.Status)
	default:
		return fmt.Sprintf("▶ %s (%s) — %d/%d — ACTIVE", node.Issue.Title, typeLabel, node.ClosedCount, node.TotalCount)
	}
}

// roadmapTypeLabel returns the issue type, appending ", parallel" when the
// node carries a "parallel" label.
func roadmapTypeLabel(node *types.RoadmapNode) string {
	label := string(node.Issue.IssueType)
	if hasLabel(node.Issue.Labels, "parallel") {
		label += ", parallel"
	}
	return label
}
