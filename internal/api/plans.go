package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/sentiolabs/arc/internal/workspace"
)

// --- Request/Response types ---

// setPlanRequest is the request body for setting an inline plan.
type setPlanRequest struct {
	Text string `json:"text"`
}

// createPlanRequest is the request body for creating a shared plan.
type createPlanRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// updatePlanRequest is the request body for updating a shared plan.
type updatePlanRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// linkPlanRequest is the request body for linking issues to a plan.
type linkPlanRequest struct {
	IssueIDs []string `json:"issue_ids"`
}

// --- Inline Plan Handlers ---

// setIssuePlan sets or updates the inline plan for an issue.
func (s *Server) setIssuePlan(c echo.Context) error {
	id := c.Param("id")
	actor := getActor(c)

	// Validate issue belongs to workspace
	if err := s.validateIssueWorkspace(c, id); err != nil {
		if errors.Is(err, errWorkspaceMismatch) {
			return errorJSON(c, http.StatusForbidden, "access denied")
		}
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	var req setPlanRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	if req.Text == "" {
		return errorJSON(c, http.StatusBadRequest, "plan text is required")
	}

	plan, err := s.store.SetInlinePlan(c.Request().Context(), id, actor, req.Text)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return createdJSON(c, plan)
}

// getIssuePlan returns the plan context for an issue.
func (s *Server) getIssuePlan(c echo.Context) error {
	id := c.Param("id")

	// Validate issue belongs to workspace
	if err := s.validateIssueWorkspace(c, id); err != nil {
		if errors.Is(err, errWorkspaceMismatch) {
			return errorJSON(c, http.StatusForbidden, "access denied")
		}
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	ctx := c.Request().Context()
	planContext, err := s.store.GetPlanContext(ctx, id)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, planContext)
}

// getIssuePlanHistory returns the plan version history for an issue.
func (s *Server) getIssuePlanHistory(c echo.Context) error {
	id := c.Param("id")

	// Validate issue belongs to workspace
	if err := s.validateIssueWorkspace(c, id); err != nil {
		if errors.Is(err, errWorkspaceMismatch) {
			return errorJSON(c, http.StatusForbidden, "access denied")
		}
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	history, err := s.store.GetPlanHistory(c.Request().Context(), id)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, history)
}

// --- Shared Plan Handlers ---

// listPlans returns all shared plans in a workspace.
func (s *Server) listPlans(c echo.Context) error {
	wsID := workspaceID(c)

	plans, err := s.store.ListPlans(c.Request().Context(), wsID)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, plans)
}

// createPlan creates a new shared plan.
func (s *Server) createPlan(c echo.Context) error {
	wsID := workspaceID(c)

	var req createPlanRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	if req.Title == "" {
		return errorJSON(c, http.StatusBadRequest, "title is required")
	}

	plan := &types.Plan{
		ID:          workspace.GeneratePlanID(req.Title),
		WorkspaceID: wsID,
		Title:       req.Title,
		Content:     req.Content,
	}

	if err := s.store.CreatePlan(c.Request().Context(), plan); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return createdJSON(c, plan)
}

// getPlan returns a shared plan by ID.
func (s *Server) getPlan(c echo.Context) error {
	wsID := workspaceID(c)
	planID := c.Param("pid")

	ctx := c.Request().Context()
	plan, err := s.store.GetPlan(ctx, planID)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	// Validate plan belongs to workspace
	if plan.WorkspaceID != wsID {
		return errorJSON(c, http.StatusForbidden, "access denied")
	}

	// Get linked issues
	linkedIssues, err := s.store.GetLinkedIssues(ctx, planID)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}
	plan.LinkedIssues = linkedIssues

	return successJSON(c, plan)
}

// updatePlan updates a shared plan.
func (s *Server) updatePlan(c echo.Context) error {
	wsID := workspaceID(c)
	planID := c.Param("pid")

	ctx := c.Request().Context()

	// Validate plan exists and belongs to workspace
	plan, err := s.store.GetPlan(ctx, planID)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}
	if plan.WorkspaceID != wsID {
		return errorJSON(c, http.StatusForbidden, "access denied")
	}

	var req updatePlanRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	// Use existing values if not provided
	title := req.Title
	if title == "" {
		title = plan.Title
	}
	content := req.Content

	if err := s.store.UpdatePlan(ctx, planID, title, content); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	// Return updated plan
	plan.Title = title
	plan.Content = content
	plan.UpdatedAt = time.Now()

	return successJSON(c, plan)
}

// deletePlan deletes a shared plan.
func (s *Server) deletePlan(c echo.Context) error {
	wsID := workspaceID(c)
	planID := c.Param("pid")

	ctx := c.Request().Context()

	// Validate plan exists and belongs to workspace
	plan, err := s.store.GetPlan(ctx, planID)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}
	if plan.WorkspaceID != wsID {
		return errorJSON(c, http.StatusForbidden, "access denied")
	}

	if err := s.store.DeletePlan(ctx, planID); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// linkIssuesToPlan links one or more issues to a plan.
func (s *Server) linkIssuesToPlan(c echo.Context) error {
	wsID := workspaceID(c)
	planID := c.Param("pid")

	ctx := c.Request().Context()

	// Validate plan exists and belongs to workspace
	plan, err := s.store.GetPlan(ctx, planID)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}
	if plan.WorkspaceID != wsID {
		return errorJSON(c, http.StatusForbidden, "access denied")
	}

	var req linkPlanRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	if len(req.IssueIDs) == 0 {
		return errorJSON(c, http.StatusBadRequest, "at least one issue_id is required")
	}

	// Link each issue
	for _, issueID := range req.IssueIDs {
		// Validate issue belongs to workspace
		issue, err := s.store.GetIssue(ctx, issueID)
		if err != nil {
			return errorJSON(c, http.StatusNotFound, "issue not found: "+issueID)
		}
		if issue.WorkspaceID != wsID {
			return errorJSON(c, http.StatusForbidden, "issue not in workspace: "+issueID)
		}

		if err := s.store.LinkIssueToPlan(ctx, issueID, planID); err != nil {
			return errorJSON(c, http.StatusInternalServerError, err.Error())
		}
	}

	return c.NoContent(http.StatusNoContent)
}

// unlinkIssueFromPlan removes a link between an issue and a plan.
func (s *Server) unlinkIssueFromPlan(c echo.Context) error {
	wsID := workspaceID(c)
	planID := c.Param("pid")
	issueID := c.Param("id")

	ctx := c.Request().Context()

	// Validate plan exists and belongs to workspace
	plan, err := s.store.GetPlan(ctx, planID)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}
	if plan.WorkspaceID != wsID {
		return errorJSON(c, http.StatusForbidden, "access denied")
	}

	if err := s.store.UnlinkIssueFromPlan(ctx, issueID, planID); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}
