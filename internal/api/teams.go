package api

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/types"
)

// getTeamContext returns issues grouped by their teammate:* labels.
func (s *Server) getTeamContext(c echo.Context) error {
	wsID := workspaceID(c)
	epicID := c.QueryParam("epic_id")
	ctx := c.Request().Context()

	resp := TeamContext{
		Workspace:  wsID,
		Roles:      make(map[string]TeamContextRole),
		Unassigned: []TeamContextIssue{},
	}

	var issues []*types.Issue

	if epicID != "" {
		// Fetch the epic
		epic, err := s.store.GetIssue(ctx, epicID)
		if err != nil {
			return errorJSON(c, http.StatusNotFound, "epic not found")
		}
		if epic.WorkspaceID != wsID {
			return errorJSON(c, http.StatusForbidden, "access denied")
		}

		resp.Epic = &TeamContextEpic{
			ID:    epic.ID,
			Title: epic.Title,
		}

		// Get epic's inline plan
		pc, err := s.store.GetPlanContext(ctx, epicID)
		if err == nil && pc.InlinePlan != nil {
			resp.Epic.Plan = &pc.InlinePlan.Text
		}

		// Find children via dependents (issues that have parent-child dep on this epic)
		dependents, err := s.store.GetDependents(ctx, epicID)
		if err != nil {
			return errorJSON(c, http.StatusInternalServerError, err.Error())
		}

		for _, dep := range dependents {
			if dep.Type != types.DepParentChild {
				continue
			}
			child, err := s.store.GetIssue(ctx, dep.IssueID)
			if err != nil {
				continue
			}
			if child.WorkspaceID == wsID {
				issues = append(issues, child)
			}
		}
	} else {
		// Fetch all non-closed issues in the workspace
		allIssues, err := s.store.ListIssues(ctx, types.IssueFilter{
			WorkspaceID: wsID,
			Limit:       500,
		})
		if err != nil {
			return errorJSON(c, http.StatusInternalServerError, err.Error())
		}

		// Filter to only non-closed issues
		for _, issue := range allIssues {
			if issue.Status != types.StatusClosed {
				issues = append(issues, issue)
			}
		}
	}

	if len(issues) == 0 {
		return successJSON(c, resp)
	}

	// Collect issue IDs for batch label fetch
	issueIDs := make([]string, len(issues))
	for i, issue := range issues {
		issueIDs[i] = issue.ID
	}

	// Batch fetch labels for all issues
	labelsMap, err := s.store.GetLabelsForIssues(ctx, issueIDs)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	// Group issues by teammate:* label
	for _, issue := range issues {
		labels := labelsMap[issue.ID]
		tci := TeamContextIssue{
			ID:       issue.ID,
			Title:    issue.Title,
			Priority: issue.Priority,
			Status:   string(issue.Status),
			Type:     string(issue.IssueType),
		}

		// Get inline plan for this issue
		pc, err := s.store.GetPlanContext(ctx, issue.ID)
		if err == nil && pc.InlinePlan != nil {
			tci.Plan = &pc.InlinePlan.Text
		}

		// Get dependencies
		deps, err := s.store.GetDependencies(ctx, issue.ID)
		if err == nil && len(deps) > 0 {
			depIDs := make([]string, 0, len(deps))
			for _, dep := range deps {
				depIDs = append(depIDs, dep.DependsOnID)
			}
			tci.Deps = &depIDs
		}

		// Extract teammate role from labels
		role := ""
		for _, l := range labels {
			if strings.HasPrefix(l, "teammate:") {
				role = strings.TrimPrefix(l, "teammate:")
				break
			}
		}

		if role == "" {
			if epicID != "" {
				// With epic filter, unassigned children are included
				resp.Unassigned = append(resp.Unassigned, tci)
			}
			// Without epic filter, skip issues without teammate labels
			continue
		}

		r := resp.Roles[role]
		r.Issues = append(r.Issues, tci)
		resp.Roles[role] = r
	}

	return successJSON(c, resp)
}
