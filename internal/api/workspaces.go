package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/types"
)

// createWorkspaceRequest is the request body for creating a workspace.
type createWorkspaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Prefix      string `json:"prefix"`
}

// updateWorkspaceRequest is the request body for updating a workspace.
type updateWorkspaceRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// listWorkspaces returns all workspaces registered in the system.
// Workspaces are the top-level containers for organizing issues.
func (s *Server) listWorkspaces(c echo.Context) error {
	workspaces, err := s.store.ListWorkspaces(c.Request().Context())
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, workspaces)
}

// createWorkspace creates a new workspace with the specified name and prefix.
// The prefix is used to generate issue IDs within the workspace.
func (s *Server) createWorkspace(c echo.Context) error {
	var req createWorkspaceRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	ws := &types.Workspace{
		Name:        req.Name,
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

// updateWorkspace updates a workspace's name or description.
// Only non-empty fields in the request body are applied.
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

// mergeWorkspacesRequest is the request body for merging workspaces.
type mergeWorkspacesRequest struct {
	TargetID  string   `json:"target_id"`
	SourceIDs []string `json:"source_ids"`
}

// mergeWorkspaces merges one or more source workspaces into a target workspace.
// All issues and plans from the source workspaces are moved to the target,
// and the source workspaces are deleted.
func (s *Server) mergeWorkspaces(c echo.Context) error {
	var req mergeWorkspacesRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}
	if req.TargetID == "" {
		return errorJSON(c, http.StatusBadRequest, "target_id is required")
	}
	if len(req.SourceIDs) == 0 {
		return errorJSON(c, http.StatusBadRequest, "at least one source_id is required")
	}

	result, err := s.store.MergeWorkspaces(c.Request().Context(), req.TargetID, req.SourceIDs, getActor(c))
	if err != nil {
		return errorJSON(c, http.StatusBadRequest, err.Error())
	}

	// Best-effort cleanup of project configs for deleted source workspaces
	arcHome := project.DefaultArcHome()
	for _, srcID := range result.SourcesDeleted {
		if removed, cleanErr := project.CleanupWorkspaceConfigs(arcHome, srcID); cleanErr != nil {
			log.Printf("Warning: failed to clean up project configs for merged workspace %s: %v", srcID, cleanErr)
		} else if removed > 0 {
			log.Printf("Cleaned up %d project config(s) for merged workspace %s", removed, srcID)
		}
	}

	return successJSON(c, result)
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
