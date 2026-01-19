// Package api provides the REST API server for arc.
package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sentiolabs/arc/internal/storage"
)

// Server represents the REST API server.
type Server struct {
	echo    *echo.Echo
	store   storage.Storage
	address string
}

// Config holds server configuration.
type Config struct {
	Address string // e.g., ":7432" or "localhost:7432"
	Store   storage.Storage
}

// New creates a new API server.
func New(cfg Config) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	s := &Server{
		echo:    e,
		store:   cfg.Store,
		address: cfg.Address,
	}

	// Register routes
	s.registerRoutes()

	return s
}

// Start starts the server.
func (s *Server) Start() error {
	return s.echo.Start(s.address)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.echo.Shutdown(ctx)
}

// registerRoutes sets up all API routes.
func (s *Server) registerRoutes() {
	// Health check
	s.echo.GET("/health", s.healthCheck)

	// API v1 routes
	v1 := s.echo.Group("/api/v1")

	// Workspaces
	v1.GET("/workspaces", s.listWorkspaces)
	v1.POST("/workspaces", s.createWorkspace)
	v1.GET("/workspaces/:id", s.getWorkspace)
	v1.PUT("/workspaces/:id", s.updateWorkspace)
	v1.DELETE("/workspaces/:id", s.deleteWorkspace)
	v1.GET("/workspaces/:id/stats", s.getWorkspaceStats)

	// Issues (workspace-scoped)
	ws := v1.Group("/workspaces/:ws")
	ws.GET("/issues", s.listIssues)
	ws.POST("/issues", s.createIssue)
	ws.GET("/issues/:id", s.getIssue)
	ws.PUT("/issues/:id", s.updateIssue)
	ws.DELETE("/issues/:id", s.deleteIssue)
	ws.POST("/issues/:id/close", s.closeIssue)
	ws.POST("/issues/:id/reopen", s.reopenIssue)

	// Ready work and blocked issues
	ws.GET("/ready", s.getReadyWork)
	ws.GET("/blocked", s.getBlockedIssues)

	// Dependencies
	ws.GET("/issues/:id/deps", s.getDependencies)
	ws.POST("/issues/:id/deps", s.addDependency)
	ws.DELETE("/issues/:id/deps/:dep", s.removeDependency)

	// Labels
	ws.GET("/labels", s.listLabels)
	ws.POST("/labels", s.createLabel)
	ws.PUT("/labels/:name", s.updateLabel)
	ws.DELETE("/labels/:name", s.deleteLabel)
	ws.POST("/issues/:id/labels", s.addLabelToIssue)
	ws.DELETE("/issues/:id/labels/:label", s.removeLabelFromIssue)

	// Comments
	ws.GET("/issues/:id/comments", s.getComments)
	ws.POST("/issues/:id/comments", s.addComment)
	ws.PUT("/issues/:id/comments/:cid", s.updateComment)
	ws.DELETE("/issues/:id/comments/:cid", s.deleteComment)

	// Events (audit trail)
	ws.GET("/issues/:id/events", s.getEvents)
}

// healthCheck returns server health status.
func (s *Server) healthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

// Error response helper
type errorResponse struct {
	Error string `json:"error"`
}

func errorJSON(c echo.Context, status int, message string) error {
	return c.JSON(status, errorResponse{Error: message})
}

// Success response helper
func successJSON(c echo.Context, data interface{}) error {
	return c.JSON(http.StatusOK, data)
}

// Created response helper
func createdJSON(c echo.Context, data interface{}) error {
	return c.JSON(http.StatusCreated, data)
}

// getActor extracts the actor (user) from the request.
// TODO: Implement proper authentication
func getActor(c echo.Context) string {
	actor := c.Request().Header.Get("X-Actor")
	if actor == "" {
		actor = "anonymous"
	}
	return actor
}

// workspaceID extracts the workspace ID from the route params.
func workspaceID(c echo.Context) string {
	return c.Param("ws")
}

// Paginated response wrapper
type paginatedResponse struct {
	Data   interface{} `json:"data"`
	Total  int         `json:"total,omitempty"`
	Limit  int         `json:"limit,omitempty"`
	Offset int         `json:"offset,omitempty"`
}

func paginatedJSON(c echo.Context, data interface{}, total, limit, offset int) error {
	return c.JSON(http.StatusOK, paginatedResponse{
		Data:   data,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// queryInt extracts an integer query parameter with a default value.
func queryInt(c echo.Context, name string, defaultVal int) int {
	val := c.QueryParam(name)
	if val == "" {
		return defaultVal
	}
	var i int
	fmt.Sscanf(val, "%d", &i)
	return i
}
