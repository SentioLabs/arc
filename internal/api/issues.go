package api

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/types"
)

const (
	// defaultListLimit is the default number of items returned in list queries.
	defaultListLimit = 100
	// defaultPriority is the default priority for issues when parsing query parameters.
	defaultPriority = 2
)

// createIssueRequest is the request body for creating an issue.
type createIssueRequest struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
	Priority    int    `json:"priority,omitempty"`
	IssueType   string `json:"issue_type,omitempty"`
	Assignee    string `json:"assignee,omitempty"`
	ExternalRef string `json:"external_ref,omitempty"`
	ParentID    string `json:"parent_id,omitempty"` // For hierarchical child IDs
}

// updateIssueRequest is the request body for updating an issue.
type updateIssueRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Status      *string `json:"status,omitempty"`
	Priority    *int    `json:"priority,omitempty"`
	IssueType   *string `json:"issue_type,omitempty"`
	Assignee    *string `json:"assignee,omitempty"`
	ExternalRef *string `json:"external_ref,omitempty"`
}

// closeIssueRequest is the request body for closing an issue.
type closeIssueRequest struct {
	Reason string `json:"reason,omitempty"`
}

// listIssues returns issues for a workspace with optional filtering and pagination.
// Supports filtering by status, type, assignee, priority, and free-text query.
// Results include batch-fetched labels for each issue.
func (s *Server) listIssues(c echo.Context) error {
	wsID := workspaceID(c)

	filter := types.IssueFilter{
		WorkspaceID: wsID,
		Limit:       queryInt(c, "limit", defaultListLimit),
		Offset:      queryInt(c, "offset", 0),
		Query:       c.QueryParam("q"),
	}

	// Parse optional filters from query parameters
	if status := c.QueryParam("status"); status != "" {
		s := types.Status(status)
		filter.Status = &s
	}
	if issueType := c.QueryParam("type"); issueType != "" {
		t := types.IssueType(issueType)
		filter.IssueType = &t
	}
	if assignee := c.QueryParam("assignee"); assignee != "" {
		filter.Assignee = &assignee
	}
	if priority := c.QueryParam("priority"); priority != "" {
		p := queryInt(c, "priority", defaultPriority)
		filter.Priority = &p
	}

	issues, err := s.store.ListIssues(c.Request().Context(), filter)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	// Fetch labels for all issues in batch
	if len(issues) > 0 {
		issueIDs := make([]string, len(issues))
		for i, issue := range issues {
			issueIDs[i] = issue.ID
		}

		labelsMap, err := s.store.GetLabelsForIssues(c.Request().Context(), issueIDs)
		if err == nil {
			for _, issue := range issues {
				issue.Labels = labelsMap[issue.ID]
			}
		}
	}

	return paginatedJSON(c, issues, len(issues), filter.Limit, filter.Offset)
}

// createIssue creates a new issue in the specified workspace.
// The issue type, priority, and other fields are set from the request body.
func (s *Server) createIssue(c echo.Context) error {
	wsID := workspaceID(c)
	actor := getActor(c)

	var req createIssueRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	issue := &types.Issue{
		WorkspaceID: wsID,
		ParentID:    req.ParentID,
		Title:       req.Title,
		Description: req.Description,
		Status:      types.Status(req.Status),
		Priority:    req.Priority,
		IssueType:   types.IssueType(req.IssueType),
		Assignee:    req.Assignee,
		ExternalRef: req.ExternalRef,
	}

	if err := s.store.CreateIssue(c.Request().Context(), issue, actor); err != nil {
		return errorJSON(c, http.StatusBadRequest, err.Error())
	}

	return createdJSON(c, issue)
}

// getIssue retrieves an issue by ID, with optional detailed view including
// dependencies, labels, and comments when details=true is specified.
func (s *Server) getIssue(c echo.Context) error {
	id := c.Param("id")

	// Validate issue belongs to workspace (security: prevents cross-workspace access)
	issue, err := s.getIssueInWorkspace(c, id)
	if err != nil {
		if errors.Is(err, errWorkspaceMismatch) {
			return errorJSON(c, http.StatusForbidden, "access denied")
		}
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	// Return detailed view if requested
	if c.QueryParam("details") == "true" {
		details, err := s.store.GetIssueDetails(c.Request().Context(), id)
		if err != nil {
			return errorJSON(c, http.StatusNotFound, err.Error())
		}
		return successJSON(c, details)
	}

	return successJSON(c, issue)
}

// updateIssue applies partial updates to an issue.
// Only provided fields are updated; omitted fields remain unchanged.
func (s *Server) updateIssue(c echo.Context) error {
	id := c.Param("id")
	actor := getActor(c)

	// Validate issue belongs to workspace (security: prevents cross-workspace access)
	if err := s.validateIssueWorkspace(c, id); err != nil {
		if errors.Is(err, errWorkspaceMismatch) {
			return errorJSON(c, http.StatusForbidden, "access denied")
		}
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	var req updateIssueRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	// Build updates map
	updates := make(map[string]any)
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.IssueType != nil {
		updates["issue_type"] = *req.IssueType
	}
	if req.Assignee != nil {
		updates["assignee"] = *req.Assignee
	}
	if req.ExternalRef != nil {
		updates["external_ref"] = *req.ExternalRef
	}

	if len(updates) == 0 {
		return errorJSON(c, http.StatusBadRequest, "no updates provided")
	}

	if err := s.store.UpdateIssue(c.Request().Context(), id, updates, actor); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	// Return updated issue
	issue, err := s.store.GetIssue(c.Request().Context(), id)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, issue)
}

// deleteIssue deletes an issue.
func (s *Server) deleteIssue(c echo.Context) error {
	id := c.Param("id")

	// Validate issue belongs to workspace (security: prevents cross-workspace access)
	if err := s.validateIssueWorkspace(c, id); err != nil {
		if errors.Is(err, errWorkspaceMismatch) {
			return errorJSON(c, http.StatusForbidden, "access denied")
		}
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	if err := s.store.DeleteIssue(c.Request().Context(), id); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// closeIssue closes an issue.
func (s *Server) closeIssue(c echo.Context) error {
	id := c.Param("id")
	actor := getActor(c)

	// Validate issue belongs to workspace (security: prevents cross-workspace access)
	if err := s.validateIssueWorkspace(c, id); err != nil {
		if errors.Is(err, errWorkspaceMismatch) {
			return errorJSON(c, http.StatusForbidden, "access denied")
		}
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	var req closeIssueRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	if err := s.store.CloseIssue(c.Request().Context(), id, req.Reason, actor); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	// Return updated issue
	issue, err := s.store.GetIssue(c.Request().Context(), id)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, issue)
}

// reopenIssue reopens a closed issue.
func (s *Server) reopenIssue(c echo.Context) error {
	id := c.Param("id")
	actor := getActor(c)

	// Validate issue belongs to workspace (security: prevents cross-workspace access)
	if err := s.validateIssueWorkspace(c, id); err != nil {
		if errors.Is(err, errWorkspaceMismatch) {
			return errorJSON(c, http.StatusForbidden, "access denied")
		}
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	if err := s.store.ReopenIssue(c.Request().Context(), id, actor); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	// Return updated issue
	issue, err := s.store.GetIssue(c.Request().Context(), id)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, issue)
}

// getReadyWork returns issues that are ready to work on (no unresolved blockers).
// Supports filtering by type, assignee, and priority.
func (s *Server) getReadyWork(c echo.Context) error {
	wsID := workspaceID(c)

	filter := types.WorkFilter{
		WorkspaceID: wsID,
		Limit:       queryInt(c, "limit", defaultListLimit),
	}

	// Parse optional filters
	if issueType := c.QueryParam("type"); issueType != "" {
		t := types.IssueType(issueType)
		filter.IssueType = &t
	}
	if assignee := c.QueryParam("assignee"); assignee != "" {
		filter.Assignee = &assignee
	}
	if c.QueryParam("unassigned") == "true" {
		filter.Unassigned = true
	}
	if priority := c.QueryParam("priority"); priority != "" {
		p := queryInt(c, "priority", defaultPriority)
		filter.Priority = &p
	}

	issues, err := s.store.GetReadyWork(c.Request().Context(), filter)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	// Fetch labels for all issues in batch
	if len(issues) > 0 {
		issueIDs := make([]string, len(issues))
		for i, issue := range issues {
			issueIDs[i] = issue.ID
		}

		labelsMap, err := s.store.GetLabelsForIssues(c.Request().Context(), issueIDs)
		if err == nil {
			for _, issue := range issues {
				issue.Labels = labelsMap[issue.ID]
			}
		}
	}

	return successJSON(c, issues)
}

// getBlockedIssues returns issues that are blocked by unresolved dependencies.
func (s *Server) getBlockedIssues(c echo.Context) error {
	wsID := workspaceID(c)

	filter := types.WorkFilter{
		WorkspaceID: wsID,
		Limit:       queryInt(c, "limit", defaultListLimit),
	}

	issues, err := s.store.GetBlockedIssues(c.Request().Context(), filter)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	// Fetch labels for all issues in batch
	if len(issues) > 0 {
		issueIDs := make([]string, len(issues))
		for i, issue := range issues {
			issueIDs[i] = issue.ID
		}

		labelsMap, err := s.store.GetLabelsForIssues(c.Request().Context(), issueIDs)
		if err == nil {
			for _, issue := range issues {
				issue.Labels = labelsMap[issue.ID]
			}
		}
	}

	return successJSON(c, issues)
}
