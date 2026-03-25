// teams.go implements the team context API endpoint which groups issues
// by teammate role labels for coordinating agent-team workflows.
package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/types"
)

// teammatePrefix is the label prefix used to assign issues to teammates.
const teammatePrefix = "teammate:"

// teamContextIssueLimit is the max number of issues fetched for team context.
const teamContextIssueLimit = 500

// getTeamContext returns issues grouped by their teammate:* labels.
// Optionally scoped to a single epic via the epic_id query parameter.
func (s *Server) getTeamContext(c echo.Context) error {
	pID := projectID(c)
	epicID := c.QueryParam("epic_id")
	ctx := c.Request().Context()

	resp := TeamContext{
		Project:    pID,
		Roles:      make(map[string]TeamContextRole),
		Unassigned: []TeamContextIssue{},
	}

	issues, err := s.fetchTeamContextIssues(ctx, c, pID, epicID, &resp)
	if err != nil {
		return err
	}

	if len(issues) == 0 {
		return successJSON(c, resp)
	}

	if err := s.groupIssuesByRole(ctx, pID, epicID, issues, &resp); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, resp)
}

// fetchTeamContextIssues loads issues for the team context response. When epicID
// is set, it populates resp.Epic and returns the epic's children via parent-child
// dependencies. Otherwise it returns all non-closed issues in the project.
func (s *Server) fetchTeamContextIssues(
	ctx context.Context, c echo.Context, pID, epicID string, resp *TeamContext,
) ([]*types.Issue, error) {
	if epicID != "" {
		return s.fetchEpicChildIssues(ctx, c, pID, epicID, resp)
	}
	return s.fetchNonClosedIssues(ctx, c, pID)
}

// fetchEpicChildIssues loads an epic and its child issues via parent-child dependencies.
func (s *Server) fetchEpicChildIssues(
	ctx context.Context, c echo.Context, pID, epicID string, resp *TeamContext,
) ([]*types.Issue, error) {
	epic, err := s.store.GetIssue(ctx, epicID)
	if err != nil {
		return nil, errorJSON(c, http.StatusNotFound, "epic not found")
	}
	if epic.ProjectID != pID {
		return nil, errorJSON(c, http.StatusForbidden, "access denied")
	}

	resp.Epic = &TeamContextEpic{ID: epic.ID, Title: epic.Title}

	dependents, err := s.store.GetDependents(ctx, epicID)
	if err != nil {
		return nil, errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	var issues []*types.Issue
	for _, dep := range dependents {
		if dep.Type != types.DepParentChild {
			continue
		}
		child, err := s.store.GetIssue(ctx, dep.IssueID)
		if err != nil {
			continue
		}
		if child.ProjectID == pID {
			issues = append(issues, child)
		}
	}
	return issues, nil
}

// fetchNonClosedIssues loads all non-closed issues in the project,
// limited to teamContextIssueLimit results.
func (s *Server) fetchNonClosedIssues(
	ctx context.Context, c echo.Context, pID string,
) ([]*types.Issue, error) {
	allIssues, err := s.store.ListIssues(ctx, types.IssueFilter{
		ProjectID: pID,
		Limit:     teamContextIssueLimit,
	})
	if err != nil {
		return nil, errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	var issues []*types.Issue
	for _, issue := range allIssues {
		if issue.Status != types.StatusClosed {
			issues = append(issues, issue)
		}
	}
	return issues, nil
}

// groupIssuesByRole fetches labels for all issues and groups them by teammate:* label.
func (s *Server) groupIssuesByRole(
	ctx context.Context, pID, epicID string, issues []*types.Issue, resp *TeamContext,
) error {
	issueIDs := make([]string, len(issues))
	for i, issue := range issues {
		issueIDs[i] = issue.ID
	}

	labelsMap, err := s.store.GetLabelsForIssues(ctx, issueIDs)
	if err != nil {
		return err
	}

	for _, issue := range issues {
		tci := s.buildTeamContextIssue(ctx, issue)
		role := extractTeammateRole(labelsMap[issue.ID])

		if role == "" {
			if epicID != "" {
				resp.Unassigned = append(resp.Unassigned, tci)
			}
			continue
		}

		r := resp.Roles[role]
		r.Issues = append(r.Issues, tci)
		resp.Roles[role] = r
	}

	return nil
}

// buildTeamContextIssue creates a TeamContextIssue with dependency data.
func (s *Server) buildTeamContextIssue(ctx context.Context, issue *types.Issue) TeamContextIssue {
	tci := TeamContextIssue{
		ID:       issue.ID,
		Title:    issue.Title,
		Priority: issue.Priority,
		Status:   string(issue.Status),
		Type:     string(issue.IssueType),
	}

	deps, err := s.store.GetDependencies(ctx, issue.ID)
	if err == nil && len(deps) > 0 {
		depIDs := make([]string, 0, len(deps))
		for _, dep := range deps {
			depIDs = append(depIDs, dep.DependsOnID)
		}
		tci.Deps = &depIDs
	}

	return tci
}

// extractTeammateRole finds the teammate:* label and returns the role suffix.
// Returns empty string if no teammate label is found.
func extractTeammateRole(labels []string) string {
	for _, l := range labels {
		if after, ok := strings.CutPrefix(l, teammatePrefix); ok {
			return after
		}
	}
	return ""
}
