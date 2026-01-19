package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/types"
)

// createWorkspaceRequest is the request body for creating a workspace.
type createWorkspaceRequest struct {
	Name        string `json:"name"`
	Path        string `json:"path,omitempty"`
	Description string `json:"description,omitempty"`
	Prefix      string `json:"prefix"`
}

// updateWorkspaceRequest is the request body for updating a workspace.
type updateWorkspaceRequest struct {
	Name        string `json:"name,omitempty"`
	Path        string `json:"path,omitempty"`
	Description string `json:"description,omitempty"`
}

// listWorkspaces returns all workspaces.
func (s *Server) listWorkspaces(c echo.Context) error {
	workspaces, err := s.store.ListWorkspaces(c.Request().Context())
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, workspaces)
}

// createWorkspace creates a new workspace.
func (s *Server) createWorkspace(c echo.Context) error {
	var req createWorkspaceRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	ws := &types.Workspace{
		Name:        req.Name,
		Path:        req.Path,
		Description: req.Description,
		Prefix:      req.Prefix,
	}

	if err := s.store.CreateWorkspace(c.Request().Context(), ws); err != nil {
		return errorJSON(c, http.StatusBadRequest, err.Error())
	}

	return createdJSON(c, ws)
}

// getWorkspace retrieves a workspace by ID.
func (s *Server) getWorkspace(c echo.Context) error {
	id := c.Param("id")

	ws, err := s.store.GetWorkspace(c.Request().Context(), id)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	return successJSON(c, ws)
}

// updateWorkspace updates a workspace.
func (s *Server) updateWorkspace(c echo.Context) error {
	id := c.Param("id")

	var req updateWorkspaceRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	ws, err := s.store.GetWorkspace(c.Request().Context(), id)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	if req.Name != "" {
		ws.Name = req.Name
	}
	if req.Path != "" {
		ws.Path = req.Path
	}
	if req.Description != "" {
		ws.Description = req.Description
	}

	if err := s.store.UpdateWorkspace(c.Request().Context(), ws); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, ws)
}

// deleteWorkspace deletes a workspace.
func (s *Server) deleteWorkspace(c echo.Context) error {
	id := c.Param("id")

	if err := s.store.DeleteWorkspace(c.Request().Context(), id); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// getWorkspaceStats returns statistics for a workspace.
func (s *Server) getWorkspaceStats(c echo.Context) error {
	id := c.Param("id")

	stats, err := s.store.GetStatistics(c.Request().Context(), id)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, stats)
}
