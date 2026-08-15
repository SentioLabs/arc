// Package api provides HTTP handlers for the arc REST API.
// This file implements ephemeral plan management — plans are lightweight review
// artifacts backed by filesystem markdown files, with metadata and comments in the DB.
package api

import (
	"errors"
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

// File permission constants for plan files and directories.
const (
	planFilePerms = 0o600
	planDirPerms  = 0o750
)

// createPlanRequest is the body for POST /plans.
type createPlanRequest struct {
	FilePath string `json:"file_path" validate:"required"`
}

// updatePlanContentRequest is the body for PUT /plans/:planId.
type updatePlanContentRequest struct {
	Content string `json:"content" validate:"required"`
}

// updatePlanStatusRequest is the body for PATCH /plans/:planId/status.
type updatePlanStatusRequest struct {
	Status string `json:"status" validate:"required"`
}

// createPlanCommentRequest is the body for POST /plans/:planId/comments.
// LineNumber is nil for overall feedback, or a specific line for anchored comments.
type createPlanCommentRequest struct {
	LineNumber *int                     `json:"line_number,omitempty"`
	Content    string                   `json:"content" validate:"required"`
	Anchor     *types.PlanCommentAnchor `json:"anchor,omitempty"`
}

// updatePlanCommentRequest is the body for PATCH /plans/:planId/comments/:commentId.
// Pointer fields distinguish "omitted" (nil = unchanged) from provided values.
// Anchor semantics: omitted/null = unchanged; object = full replace.
type updatePlanCommentRequest struct {
	Content  *string                  `json:"content,omitempty"`
	Anchor   *types.PlanCommentAnchor `json:"anchor,omitempty"`
	Resolved *bool                    `json:"resolved,omitempty"`
}

// validateAnchor checks anchor invariants; a nil anchor is valid.
func validateAnchor(a *types.PlanCommentAnchor) error {
	if a == nil {
		return nil
	}
	if a.LineStart < 1 {
		return errors.New("anchor.line_start must be >= 1")
	}
	if a.LineEnd < a.LineStart {
		return errors.New("anchor.line_end must be >= anchor.line_start")
	}
	if strings.TrimSpace(a.QuotedText) == "" {
		return errors.New("anchor.quoted_text must not be empty")
	}
	if a.Occurrence < 0 {
		return errors.New("anchor.occurrence must be >= 0")
	}
	return nil
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
	if err := os.MkdirAll(dir, planDirPerms); err != nil {
		return errorJSON(c, http.StatusInternalServerError, fmt.Sprintf("creating directory: %v", err))
	}

	if err := os.WriteFile(plan.FilePath, []byte(req.Content), planFilePerms); err != nil {
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
	case types.PlanStatusDraft, types.PlanStatusInReview, types.PlanStatusApproved,
		types.PlanStatusRejected, types.PlanStatusChangesRequested:
		// valid
	default:
		return errorJSON(c, http.StatusBadRequest,
			"status must be one of: draft, in_review, approved, rejected, changes_requested")
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

	if err := validateAnchor(req.Anchor); err != nil {
		return errorJSON(c, http.StatusBadRequest, err.Error())
	}

	lineNumber := req.LineNumber
	if req.Anchor != nil {
		ls := req.Anchor.LineStart
		lineNumber = &ls // mirror anchor.line_start into line_number for CLI compat
	}

	comment := &types.PlanComment{
		ID:         "pc." + project.GeneratePlanID("comment"),
		PlanID:     planID,
		LineNumber: lineNumber,
		Content:    req.Content,
		Anchor:     req.Anchor,
		CreatedAt:  time.Now(),
	}

	if err := s.store.CreatePlanComment(ctx, comment); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return createdJSON(c, comment)
}

// updatePlanComment applies a partial update to a plan review comment.
func (s *Server) updatePlanComment(c echo.Context) error {
	planID := c.Param("planId")
	commentID := c.Param("commentId")
	ctx := c.Request().Context()

	// Verify plan exists.
	if _, err := s.store.GetPlan(ctx, planID); err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	// Read-merge-write: fetch the full comment so a partial PATCH can't wipe
	// fields it doesn't mention (e.g. anchor, resolved_at).
	comment, err := s.store.GetPlanComment(ctx, commentID)
	if err != nil || comment.PlanID != planID {
		return errorJSON(c, http.StatusNotFound, "comment not found")
	}

	var req updatePlanCommentRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	// Content: omitted = unchanged; empty string is rejected as invalid.
	if req.Content != nil {
		if strings.TrimSpace(*req.Content) == "" {
			return errorJSON(c, http.StatusBadRequest, "content must not be empty")
		}
		comment.Content = *req.Content
	}
	// Anchor: omitted/null = unchanged; object = full replace + re-mirror line_number.
	if req.Anchor != nil {
		if err := validateAnchor(req.Anchor); err != nil {
			return errorJSON(c, http.StatusBadRequest, err.Error())
		}
		comment.Anchor = req.Anchor
		ls := req.Anchor.LineStart
		comment.LineNumber = &ls
	}
	// Resolved: true sets resolved_at, false clears it.
	if req.Resolved != nil {
		if *req.Resolved {
			now := time.Now()
			comment.ResolvedAt = &now
		} else {
			comment.ResolvedAt = nil
		}
	}
	now := time.Now()
	comment.UpdatedAt = &now

	if err := s.store.UpdatePlanComment(ctx, comment); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}
	return successJSON(c, comment)
}

// deletePlanComment removes a plan review comment.
func (s *Server) deletePlanComment(c echo.Context) error {
	planID := c.Param("planId")
	commentID := c.Param("commentId")
	ctx := c.Request().Context()

	comment, err := s.store.GetPlanComment(ctx, commentID)
	if err != nil || comment.PlanID != planID {
		return errorJSON(c, http.StatusNotFound, "comment not found")
	}

	if err := s.store.DeletePlanComment(ctx, commentID); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

// validateFilePath checks that a file path is within the current working directory.
func (s *Server) validateFilePath(filePath string) error {
	if filePath == "" {
		return errors.New("file_path is required")
	}
	if !filepath.IsAbs(filePath) {
		return errors.New("file_path must be absolute")
	}
	// Basic path traversal check: reject paths containing ".." components.
	cleaned := filepath.Clean(filePath)
	if strings.Contains(cleaned, "..") {
		return errors.New("path must not contain '..' components")
	}
	return nil
}
