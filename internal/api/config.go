// Package api — config endpoints serve the arc config TOML file via REST.
// Used by the web Settings page; CLI talks to disk directly.
package api

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	cfgpkg "github.com/sentiolabs/arc/internal/config"
)

type configMeta struct {
	Path            string   `json:"path"`
	RequiresRestart []string `json:"requires_restart"`
}

type configResponse struct {
	*cfgpkg.Config
	Meta configMeta `json:"meta"`
}

type configValidationErrorBody struct {
	Errors map[string]string `json:"errors"`
}

// TODO(security): require auth before binding non-loopback. The config
// surface is currently safe only because the server is localhost-bound by
// default.

func (s *Server) getConfig(c echo.Context) error {
	path := cfgpkg.DefaultPath()
	cfg, err := cfgpkg.Load(path)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}
	return successJSON(c, configResponse{
		Config: cfg,
		Meta:   configMeta{Path: path, RequiresRestart: cfgpkg.RequiresRestart()},
	})
}

func (s *Server) putConfig(c echo.Context) error {
	var incoming cfgpkg.Config
	if err := c.Bind(&incoming); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}
	if err := cfgpkg.Validate(&incoming); err != nil {
		var ve cfgpkg.ValidationError
		if errors.As(err, &ve) {
			return c.JSON(http.StatusBadRequest, configValidationErrorBody{Errors: ve})
		}
		return errorJSON(c, http.StatusBadRequest, err.Error())
	}
	path := cfgpkg.DefaultPath()
	if err := cfgpkg.Save(path, &incoming); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}
	return successJSON(c, configResponse{
		Config: &incoming,
		Meta:   configMeta{Path: path, RequiresRestart: cfgpkg.RequiresRestart()},
	})
}
