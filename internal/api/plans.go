package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/project"
	"github.com/sentiolabs/arc/internal/types"
)

// --- Request/Response types ---

// setPlanRequest is the request body for setting a plan on an issue.
type setPlanRequest struct {
	Text   string `json:"text"`
	Status string `json:"status"`
}

// updatePlanRequest is the request body for updating a plan's content.
type updatePlanRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// updateStatusRequest is the request body for updating a plan's status.
type updateStatusRequest struct {
	Status string `json:"status"`
}

// --- Plan Handlers ---

// setIssuePlan creates or updates the plan for an issue.
func (s *Server) setIssuePlan(c echo.Context) error {
	id := c.Param("id")
	wsID := workspaceID(c)

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

	// Default status to "draft" if not specified
	status := req.Status
	if status == "" {
		status = types.PlanStatusDraft
	}

	// Get issue title for plan title generation
	issue, err := s.store.GetIssue(c.Request().Context(), id)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	plan := &types.Plan{
		ID:        project.GeneratePlanID(issue.Title),
		ProjectID: wsID,
		Title:     "Plan for: " + issue.Title,
		Content:   req.Text,
		Status:    status,
		IssueID:   id,
	}

	if err := s.store.CreateOrUpdatePlan(c.Request().Context(), plan); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return createdJSON(c, plan)
}

// getIssuePlan returns the plan for an issue.
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
	plan, err := s.store.GetPlanByIssueID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "no plan found") {
			return errorJSON(c, http.StatusNotFound, "no plan found for issue")
		}
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, plan)
}

// listPlans returns all plans in a workspace, optionally filtered by status.
func (s *Server) listPlans(c echo.Context) error {
	wsID := workspaceID(c)
	status := c.QueryParam("status")

	plans, err := s.store.ListPlans(c.Request().Context(), wsID, status)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, plans)
}

// getPlan returns a plan by ID.
func (s *Server) getPlan(c echo.Context) error {
	wsID := workspaceID(c)
	planID := c.Param("pid")

	ctx := c.Request().Context()
	plan, err := s.store.GetPlan(ctx, planID)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	// Validate plan belongs to workspace
	if plan.ProjectID != wsID {
		return errorJSON(c, http.StatusForbidden, "access denied")
	}

	return successJSON(c, plan)
}

// updatePlan updates a plan's content.
func (s *Server) updatePlan(c echo.Context) error {
	wsID := workspaceID(c)
	planID := c.Param("pid")

	ctx := c.Request().Context()

	// Validate plan exists and belongs to workspace
	plan, err := s.store.GetPlan(ctx, planID)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}
	if plan.ProjectID != wsID {
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

	if err := s.store.UpdatePlanContent(ctx, planID, title, content); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	// Return updated plan
	plan.Title = title
	plan.Content = content

	return successJSON(c, plan)
}

// deletePlan deletes a plan.
func (s *Server) deletePlan(c echo.Context) error {
	wsID := workspaceID(c)
	planID := c.Param("pid")

	ctx := c.Request().Context()

	// Validate plan exists and belongs to workspace
	plan, err := s.store.GetPlan(ctx, planID)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}
	if plan.ProjectID != wsID {
		return errorJSON(c, http.StatusForbidden, "access denied")
	}

	if err := s.store.DeletePlan(ctx, planID); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// updatePlanStatus updates the status of a plan.
func (s *Server) updatePlanStatus(c echo.Context) error {
	wsID := workspaceID(c)
	planID := c.Param("pid")

	ctx := c.Request().Context()

	// Validate plan exists and belongs to workspace
	plan, err := s.store.GetPlan(ctx, planID)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}
	if plan.ProjectID != wsID {
		return errorJSON(c, http.StatusForbidden, "access denied")
	}

	var req updateStatusRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	// Validate status
	switch req.Status {
	case types.PlanStatusDraft, types.PlanStatusApproved, types.PlanStatusRejected:
		// valid
	default:
		return errorJSON(c, http.StatusBadRequest, "status must be one of: draft, approved, rejected")
	}

	if err := s.store.UpdatePlanStatus(ctx, planID, req.Status); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	plan.Status = req.Status

	return successJSON(c, plan)
}

// getPendingCount returns the count of draft plans in a workspace.
func (s *Server) getPendingCount(c echo.Context) error {
	wsID := workspaceID(c)

	count, err := s.store.CountPlansByStatus(c.Request().Context(), wsID, types.PlanStatusDraft)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, map[string]int{"count": count})
}
