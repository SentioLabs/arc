//go:build !webui

// Package web provides embedded static files for the Arc web UI.
// This stub is used when building without the webui tag.
package web

import (
	"github.com/labstack/echo/v4"
)

// Enabled indicates whether the web UI was compiled into this build.
const Enabled = false

// RegisterSPA is a no-op when built without the webui tag.
// The web UI is not available in this build.
func RegisterSPA(e *echo.Echo) {
	// No-op: web UI not embedded in this build
}
