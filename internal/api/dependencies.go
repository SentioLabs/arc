package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/types"
)

// addDependencyRequest is the request body for adding a dependency.
type addDependencyRequest struct {
	DependsOnID string `json:"depends_on_id"`
	Type        string `json:"type"` // blocks, parent-child, related, discovered-from
}

// getDependencies returns an issue's dependencies.
func (s *Server) getDependencies(c echo.Context) error {
	id := c.Param("id")

	// Validate issue belongs to workspace (security: prevents cross-workspace access)
	if err := s.validateIssueWorkspace(c, id); err != nil {
		if err == errWorkspaceMismatch {
			return errorJSON(c, http.StatusForbidden, "access denied")
		}
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	deps, err := s.store.GetDependencies(c.Request().Context(), id)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	dependents, err := s.store.GetDependents(c.Request().Context(), id)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, map[string]interface{}{
		"dependencies": deps,
		"dependents":   dependents,
	})
}

// addDependency adds a dependency to an issue.
func (s *Server) addDependency(c echo.Context) error {
	id := c.Param("id")
	actor := getActor(c)

	// Validate issue belongs to workspace (security: prevents cross-workspace access)
	if err := s.validateIssueWorkspace(c, id); err != nil {
		if err == errWorkspaceMismatch {
			return errorJSON(c, http.StatusForbidden, "access denied")
		}
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	var req addDependencyRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	dep := &types.Dependency{
		IssueID:     id,
		DependsOnID: req.DependsOnID,
		Type:        types.DependencyType(req.Type),
	}

	if err := s.store.AddDependency(c.Request().Context(), dep, actor); err != nil {
		return errorJSON(c, http.StatusBadRequest, err.Error())
	}

	return createdJSON(c, dep)
}

// removeDependency removes a dependency from an issue.
func (s *Server) removeDependency(c echo.Context) error {
	id := c.Param("id")
	depID := c.Param("dep")
	actor := getActor(c)

	// Validate issue belongs to workspace (security: prevents cross-workspace access)
	if err := s.validateIssueWorkspace(c, id); err != nil {
		if err == errWorkspaceMismatch {
			return errorJSON(c, http.StatusForbidden, "access denied")
		}
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	if err := s.store.RemoveDependency(c.Request().Context(), id, depID, actor); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}
