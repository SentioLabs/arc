package api

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/sentiolabs/arc/internal/config"
)

// Per-project config endpoints expose a generic key/value store scoped to a
// project. The API stores keys verbatim; semantic validation (for example of
// docs.* keys) belongs to the callers that own those namespaces.

// setProjectConfigRequest is the request body for upserting a config key.
type setProjectConfigRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// getProjectConfig returns all per-project config key/value pairs.
func (s *Server) getProjectConfig(c echo.Context) error {
	id := c.Param("id")

	if _, err := s.store.GetProject(c.Request().Context(), id); err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	values, err := s.store.GetProjectConfig(c.Request().Context(), id)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, map[string]any{"config": values})
}

// putProjectConfig upserts a single per-project config key.
// An existing key is overwritten with the new value.
func (s *Server) putProjectConfig(c echo.Context) error {
	id := c.Param("id")

	var req setProjectConfigRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}
	if req.Key == "" {
		return errorJSON(c, http.StatusBadRequest, "key is required")
	}

	// Validate docs.* namespace values server-side; other keys are stored verbatim.
	switch req.Key {
	case config.ProjectDocsTypeKey:
		if !config.ValidDocsType(req.Value) {
			return errorJSON(c, http.StatusBadRequest, "invalid docs type (want markdown or obsidian)")
		}
	case config.ProjectDocsPathKey:
		if strings.Contains(req.Value, "..") {
			return errorJSON(c, http.StatusBadRequest, "docs path must not contain '..'")
		}
	}

	if _, err := s.store.GetProject(c.Request().Context(), id); err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	if err := s.store.SetProjectConfig(c.Request().Context(), id, req.Key, req.Value); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, req)
}

// deleteProjectConfig removes a single per-project config key.
func (s *Server) deleteProjectConfig(c echo.Context) error {
	id := c.Param("id")
	key := c.Param("key")

	if _, err := s.store.GetProject(c.Request().Context(), id); err != nil {
		return errorJSON(c, http.StatusNotFound, err.Error())
	}

	if err := s.store.DeleteProjectConfig(c.Request().Context(), id, key); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, map[string]any{"deleted": key})
}
