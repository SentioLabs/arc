package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/types"
)

// createLabelRequest is the request body for creating a label.
type createLabelRequest struct {
	Name        string `json:"name"`
	Color       string `json:"color,omitempty"`
	Description string `json:"description,omitempty"`
}

// updateLabelRequest is the request body for updating a label.
type updateLabelRequest struct {
	Color       string `json:"color,omitempty"`
	Description string `json:"description,omitempty"`
}

// addLabelToIssueRequest is the request body for adding a label to an issue.
type addLabelToIssueRequest struct {
	Label string `json:"label"`
}

// listLabels returns all labels for a workspace.
func (s *Server) listLabels(c echo.Context) error {
	wsID := workspaceID(c)

	labels, err := s.store.ListLabels(c.Request().Context(), wsID)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, labels)
}

// createLabel creates a new label.
func (s *Server) createLabel(c echo.Context) error {
	wsID := workspaceID(c)

	var req createLabelRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	label := &types.Label{
		WorkspaceID: wsID,
		Name:        req.Name,
		Color:       req.Color,
		Description: req.Description,
	}

	if err := s.store.CreateLabel(c.Request().Context(), label); err != nil {
		return errorJSON(c, http.StatusBadRequest, err.Error())
	}

	return createdJSON(c, label)
}

// updateLabel updates a label.
func (s *Server) updateLabel(c echo.Context) error {
	wsID := workspaceID(c)
	name := c.Param("name")

	var req updateLabelRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	label, err := s.store.GetLabel(c.Request().Context(), wsID, name)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	if req.Color != "" {
		label.Color = req.Color
	}
	if req.Description != "" {
		label.Description = req.Description
	}

	if err := s.store.UpdateLabel(c.Request().Context(), label); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, label)
}

// deleteLabel deletes a label.
func (s *Server) deleteLabel(c echo.Context) error {
	wsID := workspaceID(c)
	name := c.Param("name")

	if err := s.store.DeleteLabel(c.Request().Context(), wsID, name); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// addLabelToIssue adds a label to an issue.
func (s *Server) addLabelToIssue(c echo.Context) error {
	id := c.Param("id")
	actor := getActor(c)

	var req addLabelToIssueRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	if err := s.store.AddLabelToIssue(c.Request().Context(), id, req.Label, actor); err != nil {
		return errorJSON(c, http.StatusBadRequest, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// removeLabelFromIssue removes a label from an issue.
func (s *Server) removeLabelFromIssue(c echo.Context) error {
	id := c.Param("id")
	label := c.Param("label")
	actor := getActor(c)

	if err := s.store.RemoveLabelFromIssue(c.Request().Context(), id, label, actor); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}
