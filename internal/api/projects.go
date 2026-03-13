package api

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
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
	projects, err := s.store.ListProjects(c.Request().Context())
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, projects)
}

// createProject creates a new project with the specified name and prefix.
// The prefix is used to generate issue IDs within the project.
func (s *Server) createProject(c echo.Context) error {
	var req createProjectRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	p := &types.Project{
		Name:        req.Name,
		Description: req.Description,
		Prefix:      req.Prefix,
	}

	if err := s.store.CreateProject(c.Request().Context(), p); err != nil {
		return errorJSON(c, http.StatusBadRequest, err.Error())
	}

	return createdJSON(c, p)
}

// getProject retrieves a project by ID.
func (s *Server) getProject(c echo.Context) error {
	id := c.Param("id")

	p, err := s.store.GetProject(c.Request().Context(), id)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	return successJSON(c, p)
}

// updateProject updates a project's name or description.
// Only non-empty fields in the request body are applied.
func (s *Server) updateProject(c echo.Context) error {
	id := c.Param("id")

	var req updateProjectRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	p, err := s.store.GetProject(c.Request().Context(), id)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	if req.Name != "" && req.Name != p.Name {
		// Check for duplicate name
		existing, _ := s.store.GetProjectByName(c.Request().Context(), req.Name)
		if existing != nil {
			return errorJSON(c, http.StatusConflict, "a project with that name already exists")
		}
		p.Name = req.Name
	}
	if req.Description != "" {
		p.Description = req.Description
	}

	if err := p.Validate(); err != nil {
		return errorJSON(c, http.StatusBadRequest, err.Error())
	}

	if err := s.store.UpdateProject(c.Request().Context(), p); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, p)
}

// deleteProject deletes a project.
func (s *Server) deleteProject(c echo.Context) error {
	id := c.Param("id")

	if err := s.store.DeleteProject(c.Request().Context(), id); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// mergeProjectsRequest is the request body for merging projects.
type mergeProjectsRequest struct {
	TargetID  string   `json:"target_id"`
	SourceIDs []string `json:"source_ids"`
}

// mergeProjects merges one or more source projects into a target project.
// All issues and plans from the source projects are moved to the target,
// and the source projects are deleted.
func (s *Server) mergeProjects(c echo.Context) error {
	var req mergeProjectsRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}
	if req.TargetID == "" {
		return errorJSON(c, http.StatusBadRequest, "target_id is required")
	}
	if len(req.SourceIDs) == 0 {
		return errorJSON(c, http.StatusBadRequest, "at least one source_id is required")
	}

	result, err := s.store.MergeProjects(c.Request().Context(), req.TargetID, req.SourceIDs, getActor(c))
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			return errorJSON(c, http.StatusNotFound, errMsg)
		}
		return errorJSON(c, http.StatusInternalServerError, errMsg)
	}

	return successJSON(c, result)
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
