// Package web provides embedded static files for the Arc web UI.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

//go:embed all:build
var staticFiles embed.FS

// GetFileSystem returns the embedded filesystem stripped of the "build" prefix.
func GetFileSystem() (fs.FS, error) {
	return fs.Sub(staticFiles, "build")
}

// StaticHandler returns an Echo handler that serves the embedded SPA.
// It serves static files and falls back to index.html for SPA routing.
func StaticHandler() echo.HandlerFunc {
	fsys, err := GetFileSystem()
	if err != nil {
		// This should never happen with properly embedded files
		panic("failed to get embedded filesystem: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(fsys))

	return func(c echo.Context) error {
		path := c.Request().URL.Path

		// Try to serve the file directly
		if file, err := fsys.Open(strings.TrimPrefix(path, "/")); err == nil {
			file.Close()
			fileServer.ServeHTTP(c.Response(), c.Request())
			return nil
		}

		// Fall back to index.html for SPA routing
		c.Request().URL.Path = "/"
		fileServer.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}

// RegisterSPA registers the SPA handler on the Echo instance.
// It should be called after all API routes are registered.
func RegisterSPA(e *echo.Echo) {
	// Serve static files and SPA fallback for all non-API routes
	e.GET("/*", StaticHandler())
}
