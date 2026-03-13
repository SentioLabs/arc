// Package api provides HTTP handlers for the arc REST API.
// This file contains workspace path (directory registration) endpoints.
package api

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/types"
)

// createWorkspaceRequest is the request body for creating a workspace (directory path).
type createWorkspaceRequest struct {
	Path      string `json:"path"`
	Label     string `json:"label,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
	GitRemote string `json:"git_remote,omitempty"`
	PathType  string `json:"path_type,omitempty"`
}

// updateWorkspaceRequest is the request body for updating a workspace (directory path).
type updateWorkspaceRequest struct {
	Label     string `json:"label,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
	GitRemote string `json:"git_remote,omitempty"`
	PathType  string `json:"path_type,omitempty"`
}

// listWorkspaces returns all workspaces (directory paths) for a project.
func (s *Server) listWorkspaces(c echo.Context) error {
	projectID := c.Param("id")
	ctx := c.Request().Context()

	workspaces, err := s.store.ListWorkspaces(ctx, projectID)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	if workspaces == nil {
		workspaces = []*types.Workspace{}
	}

	return successJSON(c, workspaces)
}

// createWorkspace registers a new workspace (directory path) for a project.
func (s *Server) createWorkspace(c echo.Context) error {
	projectID := c.Param("id")
	ctx := c.Request().Context()

	var req createWorkspaceRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	if req.Path == "" {
		return errorJSON(c, http.StatusBadRequest, "path is required")
	}

	ws := &types.Workspace{
		ProjectID: projectID,
		Path:      req.Path,
		Label:     req.Label,
		Hostname:  req.Hostname,
		GitRemote: req.GitRemote,
		PathType:  req.PathType,
	}

	if err := s.store.CreateWorkspace(ctx, ws); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") || strings.Contains(err.Error(), "already exists") {
			return errorJSON(c, http.StatusConflict, "path already exists for this project")
		}
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return createdJSON(c, ws)
}

// updateWorkspace updates a workspace's metadata (label, hostname, git remote, path type).
func (s *Server) updateWorkspace(c echo.Context) error {
	wsID := c.Param("pathId")
	ctx := c.Request().Context()

	var req updateWorkspaceRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	ws, err := s.store.GetWorkspace(ctx, wsID)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	if req.Label != "" {
		ws.Label = req.Label
	}
	if req.Hostname != "" {
		ws.Hostname = req.Hostname
	}
	if req.GitRemote != "" {
		ws.GitRemote = req.GitRemote
	}
	if req.PathType != "" {
		ws.PathType = req.PathType
	}

	if err := s.store.UpdateWorkspace(ctx, ws); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, ws)
}

// deleteWorkspace removes a workspace (directory path) from a project by its ID.
func (s *Server) deleteWorkspace(c echo.Context) error {
	wsID := c.Param("pathId")
	ctx := c.Request().Context()

	if err := s.store.DeleteWorkspace(ctx, wsID); err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// resolveProject finds the project associated with a filesystem path.
// It also updates the workspace's last_accessed_at timestamp.
func (s *Server) resolveProject(c echo.Context) error {
	path := c.QueryParam("path")
	ctx := c.Request().Context()

	if path == "" {
		return errorJSON(c, http.StatusBadRequest, "path query parameter is required")
	}

	ws, err := s.store.ResolveProjectByPath(ctx, path)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	// Update last_accessed_at (best-effort)
	_ = s.store.UpdateWorkspaceLastAccessed(ctx, ws.ID)

	// Look up project name for the response
	projectName := ""
	if p, err := s.store.GetProject(ctx, ws.ProjectID); err == nil {
		projectName = p.Name
	}

	return successJSON(c, types.ProjectResolution{
		ProjectID:   ws.ProjectID,
		ProjectName: projectName,
		PathID:      ws.ID,
	})
}
