package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/project"
	"github.com/sentiolabs/arc/internal/types"
)

// --- Request/Response types ---

type createPlanRequest struct {
	FilePath string `json:"file_path" validate:"required"`
}

type updatePlanContentRequest struct {
	Content string `json:"content" validate:"required"`
}

type updatePlanStatusRequest struct {
	Status string `json:"status" validate:"required"`
}

type createPlanCommentRequest struct {
	LineNumber *int   `json:"line_number,omitempty"`
	Content    string `json:"content" validate:"required"`
}

// --- Plan Handlers ---

// createPlan registers an ephemeral plan backed by a filesystem markdown file.
func (s *Server) createPlan(c echo.Context) error {
	var req createPlanRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	if req.FilePath == "" {
		return errorJSON(c, http.StatusBadRequest, "file_path is required")
	}

	if err := s.validateFilePath(req.FilePath); err != nil {
		return errorJSON(c, http.StatusBadRequest, err.Error())
	}

	now := time.Now()
	plan := &types.Plan{
		ID:        project.GeneratePlanID(filepath.Base(req.FilePath)),
		FilePath:  req.FilePath,
		Status:    types.PlanStatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.store.CreatePlan(c.Request().Context(), plan); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return createdJSON(c, plan)
}

// getPlan returns plan metadata and file content.
func (s *Server) getPlan(c echo.Context) error {
	planID := c.Param("planId")
	ctx := c.Request().Context()

	plan, err := s.store.GetPlan(ctx, planID)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	content, err := os.ReadFile(plan.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return errorJSON(c, http.StatusNotFound, "plan file not found")
		}
		return errorJSON(c, http.StatusInternalServerError, fmt.Sprintf("reading plan file: %v", err))
	}

	result := types.PlanWithContent{
		Plan:    *plan,
		Content: string(content),
	}

	return successJSON(c, result)
}

// updatePlanContent writes new content to the plan's file.
func (s *Server) updatePlanContent(c echo.Context) error {
	planID := c.Param("planId")
	ctx := c.Request().Context()

	plan, err := s.store.GetPlan(ctx, planID)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	var req updatePlanContentRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	if req.Content == "" {
		return errorJSON(c, http.StatusBadRequest, "content is required")
	}

	if err := s.validateFilePath(plan.FilePath); err != nil {
		return errorJSON(c, http.StatusBadRequest, err.Error())
	}

	// Ensure parent directory exists
	dir := filepath.Dir(plan.FilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return errorJSON(c, http.StatusInternalServerError, fmt.Sprintf("creating directory: %v", err))
	}

	if err := os.WriteFile(plan.FilePath, []byte(req.Content), 0o644); err != nil {
		return errorJSON(c, http.StatusInternalServerError, fmt.Sprintf("writing plan file: %v", err))
	}

	result := types.PlanWithContent{
		Plan:    *plan,
		Content: req.Content,
	}

	return successJSON(c, result)
}

// updatePlanStatus updates the status of a plan.
func (s *Server) updatePlanStatus(c echo.Context) error {
	planID := c.Param("planId")
	ctx := c.Request().Context()

	var req updatePlanStatusRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	// Validate status
	switch req.Status {
	case types.PlanStatusDraft, types.PlanStatusInReview, types.PlanStatusApproved, types.PlanStatusRejected:
		// valid
	default:
		return errorJSON(c, http.StatusBadRequest, "status must be one of: draft, in_review, approved, rejected")
	}

	if err := s.store.UpdatePlanStatus(ctx, planID, req.Status); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	plan, err := s.store.GetPlan(ctx, planID)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	return successJSON(c, plan)
}

// deletePlan deletes a plan and its comments.
func (s *Server) deletePlan(c echo.Context) error {
	planID := c.Param("planId")
	ctx := c.Request().Context()

	if err := s.store.DeletePlan(ctx, planID); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// listPlanComments returns all comments for a plan.
func (s *Server) listPlanComments(c echo.Context) error {
	planID := c.Param("planId")
	ctx := c.Request().Context()

	comments, err := s.store.ListPlanComments(ctx, planID)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	if comments == nil {
		comments = []*types.PlanComment{}
	}

	return successJSON(c, comments)
}

// createPlanComment adds a review comment to a plan.
func (s *Server) createPlanComment(c echo.Context) error {
	planID := c.Param("planId")
	ctx := c.Request().Context()

	// Verify plan exists
	if _, err := s.store.GetPlan(ctx, planID); err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	var req createPlanCommentRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	if req.Content == "" {
		return errorJSON(c, http.StatusBadRequest, "content is required")
	}

	comment := &types.PlanComment{
		ID:         fmt.Sprintf("pc.%s", project.GeneratePlanID("comment")),
		PlanID:     planID,
		LineNumber: req.LineNumber,
		Content:    req.Content,
		CreatedAt:  time.Now(),
	}

	if err := s.store.CreatePlanComment(ctx, comment); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return createdJSON(c, comment)
}

// validateFilePath checks that a file path is within the current working directory.
func (s *Server) validateFilePath(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("file_path is required")
	}
	if !filepath.IsAbs(filePath) {
		return fmt.Errorf("file_path must be absolute")
	}
	// Basic path traversal check: reject paths containing ".." components.
	cleaned := filepath.Clean(filePath)
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("path must not contain '..' components")
	}
	return nil
}
