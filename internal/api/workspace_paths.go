package api

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/types"
)

// createWorkspacePathRequest is the request body for creating a workspace path.
type createWorkspacePathRequest struct {
	Path      string `json:"path"`
	Label     string `json:"label,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
	GitRemote string `json:"git_remote,omitempty"`
}

// updateWorkspacePathRequest is the request body for updating a workspace path.
type updateWorkspacePathRequest struct {
	Label     string `json:"label,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
	GitRemote string `json:"git_remote,omitempty"`
}

// listWorkspacePaths returns all paths for a workspace.
func (s *Server) listWorkspacePaths(c echo.Context) error {
	wsID := c.Param("id")
	ctx := c.Request().Context()

	paths, err := s.store.ListWorkspacePaths(ctx, wsID)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	if paths == nil {
		paths = []*types.WorkspacePath{}
	}

	return successJSON(c, paths)
}

// createWorkspacePath registers a new path for a workspace.
func (s *Server) createWorkspacePath(c echo.Context) error {
	wsID := c.Param("id")
	ctx := c.Request().Context()

	var req createWorkspacePathRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	if req.Path == "" {
		return errorJSON(c, http.StatusBadRequest, "path is required")
	}

	wp := &types.WorkspacePath{
		WorkspaceID: wsID,
		Path:        req.Path,
		Label:       req.Label,
		Hostname:    req.Hostname,
		GitRemote:   req.GitRemote,
	}

	if err := s.store.CreateWorkspacePath(ctx, wp); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return errorJSON(c, http.StatusConflict, err.Error())
		}
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return createdJSON(c, wp)
}

// updateWorkspacePath updates a workspace path's metadata.
func (s *Server) updateWorkspacePath(c echo.Context) error {
	pathID := c.Param("pathId")
	ctx := c.Request().Context()

	var req updateWorkspacePathRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	wp, err := s.store.GetWorkspacePath(ctx, pathID)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	if req.Label != "" {
		wp.Label = req.Label
	}
	if req.Hostname != "" {
		wp.Hostname = req.Hostname
	}
	if req.GitRemote != "" {
		wp.GitRemote = req.GitRemote
	}

	if err := s.store.UpdateWorkspacePath(ctx, wp); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, wp)
}

// deleteWorkspacePath removes a path from a workspace.
func (s *Server) deleteWorkspacePath(c echo.Context) error {
	pathID := c.Param("pathId")
	ctx := c.Request().Context()

	if err := s.store.DeleteWorkspacePath(ctx, pathID); err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// resolveWorkspace finds the workspace associated with a filesystem path.
func (s *Server) resolveWorkspace(c echo.Context) error {
	path := c.QueryParam("path")
	ctx := c.Request().Context()

	if path == "" {
		return errorJSON(c, http.StatusBadRequest, "path query parameter is required")
	}

	wp, err := s.store.ResolveWorkspaceByPath(ctx, path)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	// Update last_accessed_at (best-effort)
	_ = s.store.UpdatePathLastAccessed(ctx, wp.ID)

	// Look up workspace name for the response
	wsName := ""
	if ws, err := s.store.GetWorkspace(ctx, wp.WorkspaceID); err == nil {
		wsName = ws.Name
	}

	return successJSON(c, types.WorkspaceResolution{
		WorkspaceID:   wp.WorkspaceID,
		WorkspaceName: wsName,
		PathID:        wp.ID,
	})
}
