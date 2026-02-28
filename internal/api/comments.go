package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// defaultEventLimit is the default number of events to return.
const defaultEventLimit = 50

// addCommentRequest is the request body for adding a comment.
type addCommentRequest struct {
	Text string `json:"text"`
}

// updateCommentRequest is the request body for updating a comment.
type updateCommentRequest struct {
	Text string `json:"text"`
}

// getComments returns comments for an issue.
func (s *Server) getComments(c echo.Context) error {
	id := c.Param("id")

	// Validate issue belongs to workspace (security: prevents cross-workspace access)
	if err := s.validateIssueWorkspace(c, id); err != nil {
		if errors.Is(err, errWorkspaceMismatch) {
			return errorJSON(c, http.StatusForbidden, "access denied")
		}
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	comments, err := s.store.GetComments(c.Request().Context(), id)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, comments)
}

// addComment adds a comment to an issue.
func (s *Server) addComment(c echo.Context) error {
	id := c.Param("id")
	actor := getActor(c)

	// Validate issue belongs to workspace (security: prevents cross-workspace access)
	if err := s.validateIssueWorkspace(c, id); err != nil {
		if errors.Is(err, errWorkspaceMismatch) {
			return errorJSON(c, http.StatusForbidden, "access denied")
		}
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	var req addCommentRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	comment, err := s.store.AddComment(c.Request().Context(), id, actor, req.Text)
	if err != nil {
		return errorJSON(c, http.StatusBadRequest, err.Error())
	}

	return createdJSON(c, comment)
}

// updateComment updates a comment.
func (s *Server) updateComment(c echo.Context) error {
	id := c.Param("id") // issue ID
	cidStr := c.Param("cid")
	cid, err := strconv.ParseInt(cidStr, 10, 64)
	if err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid comment ID")
	}

	// Validate issue belongs to workspace (security: prevents cross-workspace access)
	if err := s.validateIssueWorkspace(c, id); err != nil {
		if errors.Is(err, errWorkspaceMismatch) {
			return errorJSON(c, http.StatusForbidden, "access denied")
		}
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	var req updateCommentRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	if err := s.store.UpdateComment(c.Request().Context(), cid, req.Text); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// deleteComment deletes a comment.
func (s *Server) deleteComment(c echo.Context) error {
	id := c.Param("id") // issue ID
	cidStr := c.Param("cid")
	cid, err := strconv.ParseInt(cidStr, 10, 64)
	if err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid comment ID")
	}

	// Validate issue belongs to workspace (security: prevents cross-workspace access)
	if err := s.validateIssueWorkspace(c, id); err != nil {
		if errors.Is(err, errWorkspaceMismatch) {
			return errorJSON(c, http.StatusForbidden, "access denied")
		}
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	if err := s.store.DeleteComment(c.Request().Context(), cid); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// getEvents returns the event history for an issue.
func (s *Server) getEvents(c echo.Context) error {
	id := c.Param("id")
	limit := queryInt(c, "limit", defaultEventLimit)

	// Validate issue belongs to workspace (security: prevents cross-workspace access)
	if err := s.validateIssueWorkspace(c, id); err != nil {
		if errors.Is(err, errWorkspaceMismatch) {
			return errorJSON(c, http.StatusForbidden, "access denied")
		}
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	events, err := s.store.GetEvents(c.Request().Context(), id, limit)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, events)
}
