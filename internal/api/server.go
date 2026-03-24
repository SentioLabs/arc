// Package api provides the REST API server for arc.
package api

//go:generate go tool oapi-codegen --config=../../api/oapi-codegen.yaml ../../api/openapi.yaml

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/sentiolabs/arc/internal/version"
	"github.com/sentiolabs/arc/web"
)

// Server represents the REST API server.
type Server struct {
	echo      *echo.Echo
	store     storage.Storage
	address   string
	startTime time.Time
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
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:  true,
		LogURI:     true,
		LogMethod:  true,
		LogLatency: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			log.Printf("%s %s %d %v\n", v.Method, v.URI, v.Status, v.Latency)
			return nil
		},
	}))
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	s := &Server{
		echo:      e,
		store:     cfg.Store,
		address:   cfg.Address,
		startTime: time.Now(),
	}

	// Register routes
	s.registerRoutes()

	// Serve embedded SPA for non-API routes
	web.RegisterSPA(e)

	return s
}

// Start starts the server.
func (s *Server) Start() error {
	return s.echo.Start(s.address)
}

// Echo returns the underlying Echo instance for testing.
func (s *Server) Echo() *echo.Echo {
	return s.echo
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

	// Projects (top-level containers)
	v1.POST("/projects/merge", s.mergeProjects)
	// Resolve must be registered before :id to avoid "resolve" being captured as an ID
	v1.GET("/projects/resolve", s.resolveProject)
	v1.GET("/projects", s.listProjects)
	v1.POST("/projects", s.createProject)
	v1.GET("/projects/:id", s.getProject)
	v1.PUT("/projects/:id", s.updateProject)
	v1.DELETE("/projects/:id", s.deleteProject)
	v1.GET("/projects/:id/stats", s.getProjectStats)

	// Filesystem browser
	v1.GET("/filesystem/browse", s.browseFilesystem)

	// Workspaces (directory paths for a project)
	v1.GET("/projects/:id/workspaces", s.listWorkspaces)
	v1.POST("/projects/:id/workspaces", s.createWorkspace)
	v1.PATCH("/projects/:id/workspaces/:pathId", s.updateWorkspace)
	v1.DELETE("/projects/:id/workspaces/:pathId", s.deleteWorkspace)

	// Plans (ephemeral review artifacts, not project-scoped)
	v1.POST("/plans", s.createPlan)
	v1.GET("/plans/:planId", s.getPlan)
	v1.PUT("/plans/:planId", s.updatePlanContent)
	v1.PATCH("/plans/:planId/status", s.updatePlanStatus)
	v1.DELETE("/plans/:planId", s.deletePlan)
	v1.GET("/plans/:planId/comments", s.listPlanComments)
	v1.POST("/plans/:planId/comments", s.createPlanComment)

	// Issues (global lookup by unique ID — no project context required)
	issues := v1.Group("/issues")
	issues.GET("/:id", s.getIssue)
	issues.PUT("/:id", s.updateIssue)
	issues.DELETE("/:id", s.deleteIssue)
	issues.POST("/:id/close", s.closeIssue)
	issues.POST("/:id/reopen", s.reopenIssue)
	issues.GET("/:id/deps", s.getDependencies)
	issues.POST("/:id/deps", s.addDependency)
	issues.DELETE("/:id/deps/:dep", s.removeDependency)
	issues.POST("/:id/labels", s.addLabelToIssue)
	issues.DELETE("/:id/labels/:label", s.removeLabelFromIssue)
	issues.GET("/:id/comments", s.getComments)
	issues.POST("/:id/comments", s.addComment)
	issues.PUT("/:id/comments/:cid", s.updateComment)
	issues.DELETE("/:id/comments/:cid", s.deleteComment)
	issues.GET("/:id/events", s.getEvents)

	// Issues (project-scoped — same handlers, with workspace validation)
	ws := v1.Group("/projects/:ws")
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

	// Team context
	ws.GET("/team-context", s.getTeamContext)

	// Dependencies
	ws.GET("/issues/:id/deps", s.getDependencies)
	ws.POST("/issues/:id/deps", s.addDependency)
	ws.DELETE("/issues/:id/deps/:dep", s.removeDependency)

	// Labels (global)
	v1.GET("/labels", s.listLabels)
	v1.POST("/labels", s.createLabel)
	v1.PUT("/labels/:name", s.updateLabel)
	v1.DELETE("/labels/:name", s.deleteLabel)

	// Issue-label associations (project-scoped)
	ws.POST("/issues/:id/labels", s.addLabelToIssue)
	ws.DELETE("/issues/:id/labels/:label", s.removeLabelFromIssue)

	// Comments
	ws.GET("/issues/:id/comments", s.getComments)
	ws.POST("/issues/:id/comments", s.addComment)
	ws.PUT("/issues/:id/comments/:cid", s.updateComment)
	ws.DELETE("/issues/:id/comments/:cid", s.deleteComment)

	// Events (audit trail)
	ws.GET("/issues/:id/events", s.getEvents)

	// AI Session observability (global, not project-scoped)
	ai := v1.Group("/ai")
	ai.POST("/sessions", s.createAISession)
	ai.GET("/sessions", s.listAISessions)
	ai.GET("/sessions/:id", s.getAISession)
	ai.DELETE("/sessions/:id", s.deleteAISession)
	ai.POST("/sessions/:id/agents", s.createAIAgent)
	ai.GET("/sessions/:id/agents", s.listAIAgents)
	ai.GET("/sessions/:id/agents/:aid", s.getAIAgent)
	ai.GET("/sessions/:id/transcript", s.getSessionTranscript)
	ai.GET("/sessions/:id/agents/:aid/transcript", s.getAgentTranscript)
}

// HealthResponse contains health check information.
type HealthResponse struct {
	Status   string  `json:"status"`
	Version  string  `json:"version"`
	Uptime   float64 `json:"uptime"` // seconds
	Port     int     `json:"port"`
	WebUIURL string  `json:"webui_url"`
}

// healthCheck returns server health status.
func (s *Server) healthCheck(c echo.Context) error {
	var host string
	var port int
	if h, portStr, err := net.SplitHostPort(s.address); err == nil {
		host = h
		port, _ = strconv.Atoi(portStr)
	}
	if host == "" {
		host = "localhost"
	}
	var webuiURL string
	if port > 0 && web.Enabled {
		webuiURL = fmt.Sprintf("http://%s:%d", host, port)
	}
	return c.JSON(http.StatusOK, HealthResponse{
		Status:   "healthy",
		Version:  version.Version,
		Uptime:   time.Since(s.startTime).Seconds(),
		Port:     port,
		WebUIURL: webuiURL,
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
func successJSON(c echo.Context, data any) error {
	return c.JSON(http.StatusOK, data)
}

// Created response helper
func createdJSON(c echo.Context, data any) error {
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
	Data   any `json:"data"`
	Total  int `json:"total,omitempty"`
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

func paginatedJSON(c echo.Context, data any, total, limit, offset int) error {
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
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return i
}

// errWorkspaceMismatch is returned when an issue doesn't belong to the requested workspace.
var errWorkspaceMismatch = errors.New("issue does not belong to this workspace")

// validateIssueWorkspace fetches an issue and validates it belongs to the specified workspace.
// Returns nil if valid, or an error suitable for HTTP response.
func (s *Server) validateIssueWorkspace(c echo.Context, issueID string) error {
	_, err := s.getIssueInWorkspace(c, issueID)
	return err
}

// getIssueInWorkspace fetches an issue and validates it belongs to the specified workspace.
// When called from a project-agnostic route (no :ws param), the workspace check is skipped.
// Returns the issue if valid, or an error if not found or workspace mismatch.
func (s *Server) getIssueInWorkspace(c echo.Context, issueID string) (*types.Issue, error) {
	wsID := workspaceID(c)
	ctx := c.Request().Context()

	issue, err := s.store.GetIssue(ctx, issueID)
	if err != nil {
		return nil, err
	}

	// Skip workspace validation on project-agnostic routes (no :ws param)
	if wsID != "" && issue.ProjectID != wsID {
		return nil, errWorkspaceMismatch
	}

	return issue, nil
}
