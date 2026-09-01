package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// getRoadmap returns the project's container tree with progress and gating.
func (s *Server) getRoadmap(c echo.Context) error {
	nodes, err := s.store.GetRoadmap(c.Request().Context(), projectID(c))
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}
	return successJSON(c, nodes)
}
