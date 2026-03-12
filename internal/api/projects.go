package api

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/project"
	"github.com/sentiolabs/arc/internal/types"
)

// createProjectRequest is the request body for creating a project.
type createProjectRequest struct {
	Name        string `json:"name"`
	Path        string `json:"path,omitempty"`
	Description string `json:"description,omitempty"`
	Prefix      string `json:"prefix"`
}

// updateProjectRequest is the request body for updating a project.
type updateProjectRequest struct {
	Name        string `json:"name,omitempty"`
	Path        string `json:"path,omitempty"`
	Description string `json:"description,omitempty"`
}

// listProjects returns all projects registered in the system.
// Projects are the top-level containers for organizing issues.
func (s *Server) listProjects(c echo.Context) error {
	workspaces, err := s.store.ListWorkspaces(c.Request().Context())
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, workspaces)
}

// createProject creates a new project with the specified name and prefix.
// The prefix is used to generate issue IDs within the project.
func (s *Server) createProject(c echo.Context) error {
	var req createProjectRequest
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

// getProject retrieves a project by ID.
func (s *Server) getProject(c echo.Context) error {
	id := c.Param("id")

	ws, err := s.store.GetWorkspace(c.Request().Context(), id)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	return successJSON(c, ws)
}

// updateProject updates a project's name, path, or description.
// Only non-empty fields in the request body are applied.
func (s *Server) updateProject(c echo.Context) error {
	id := c.Param("id")

	var req updateProjectRequest
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

// deleteProject deletes a project and performs best-effort cleanup
// of any project configs that reference the deleted project.
func (s *Server) deleteProject(c echo.Context) error {
	id := c.Param("id")

	if err := s.store.DeleteWorkspace(c.Request().Context(), id); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	// Best-effort cleanup of project configs referencing this project
	arcHome := project.DefaultArcHome()
	if removed, err := project.CleanupWorkspaceConfigs(arcHome, id); err != nil {
		log.Printf("Warning: failed to clean up project configs for project %s: %v", id, err)
	} else if removed > 0 {
		log.Printf("Cleaned up %d project config(s) for deleted project %s", removed, id)
	}

	return c.NoContent(http.StatusNoContent)
}

// getProjectStats returns statistics for a project.
func (s *Server) getProjectStats(c echo.Context) error {
	id := c.Param("id")

	stats, err := s.store.GetStatistics(c.Request().Context(), id)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, stats)
}
