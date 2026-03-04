package api

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// reviewSession is the internal representation of a review session stored in memory.
// It extends the generated ReviewSession with fields not exposed via JSON or not in the OpenAPI spec.
type reviewSession struct {
	ID           string            `json:"id"`
	WorkspaceID  string            `json:"workspace_id"`
	Base         string            `json:"base"`
	Head         string            `json:"head"`
	Status       string            `json:"status"`
	Comment      string            `json:"comment,omitempty"`
	FileComments map[string]string `json:"file_comments,omitempty"`
	DiffPath     string            `json:"-"`
	Stats        *DiffStats        `json:"stats,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

// createReviewRequest is the request body for creating a review.
type createReviewRequest struct {
	Base string `json:"base"`
	Head string `json:"head"`
}

// submitReviewRequest is the request body for submitting a review decision.
type submitReviewRequest struct {
	Decision     string            `json:"decision"`
	Comment      string            `json:"comment,omitempty"`
	FileComments map[string]string `json:"file_comments,omitempty"`
}

func generateSessionID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// createReview creates a new review session by running git diff.
func (s *Server) createReview(c echo.Context) error {
	wsID := workspaceID(c)

	var req createReviewRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	if req.Base == "" || req.Head == "" {
		return errorJSON(c, http.StatusBadRequest, "base and head are required")
	}

	// Look up workspace to get path
	ws, err := s.store.GetWorkspace(c.Request().Context(), wsID)
	if err != nil {
		return errorJSON(c, http.StatusNotFound, "workspace not found")
	}

	if ws.Path == "" {
		return errorJSON(c, http.StatusBadRequest, "workspace has no path configured")
	}

	// Validate .git directory exists
	gitDir := filepath.Join(ws.Path, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return errorJSON(c, http.StatusBadRequest, "workspace path is not a git repository")
	}

	// Run git diff
	diffCmd := exec.Command("git", "diff", fmt.Sprintf("%s...%s", req.Base, req.Head))
	diffCmd.Dir = ws.Path
	diffOutput, err := diffCmd.Output()
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, fmt.Sprintf("git diff failed: %v", err))
	}

	// Run git diff --stat for stats
	statCmd := exec.Command("git", "diff", "--stat", fmt.Sprintf("%s...%s", req.Base, req.Head))
	statCmd.Dir = ws.Path
	statOutput, err := statCmd.Output()
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, fmt.Sprintf("git diff --stat failed: %v", err))
	}

	stats := parseDiffStats(string(statOutput))

	// Write diff to disk
	sessionID := generateSessionID()
	home, err := os.UserHomeDir()
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, "cannot determine home directory")
	}

	diffDir := filepath.Join(home, ".arc", "reviews", wsID)
	if err := os.MkdirAll(diffDir, 0o755); err != nil {
		return errorJSON(c, http.StatusInternalServerError, fmt.Sprintf("failed to create review directory: %v", err))
	}

	diffPath := filepath.Join(diffDir, sessionID+".diff")
	if err := os.WriteFile(diffPath, diffOutput, 0o644); err != nil {
		return errorJSON(c, http.StatusInternalServerError, fmt.Sprintf("failed to write diff: %v", err))
	}

	session := &reviewSession{
		ID:          sessionID,
		WorkspaceID: wsID,
		Base:        req.Base,
		Head:        req.Head,
		Status:      "pending",
		DiffPath:    diffPath,
		Stats:       stats,
		CreatedAt:   time.Now(),
	}

	// Store in sync.Map keyed by wsID/sessionID
	s.reviews.Store(reviewKey(wsID, sessionID), session)

	return createdJSON(c, session)
}

// getReviewDiff returns the diff file content as text/plain.
func (s *Server) getReviewDiff(c echo.Context) error {
	wsID := workspaceID(c)
	rid := c.Param("rid")

	session, ok := s.loadReview(wsID, rid)
	if !ok {
		return errorJSON(c, http.StatusNotFound, "review session not found")
	}

	data, err := os.ReadFile(session.DiffPath)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, fmt.Sprintf("failed to read diff: %v", err))
	}

	return c.Blob(http.StatusOK, "text/plain; charset=UTF-8", data)
}

// getReviewStatus returns the review session status as JSON.
func (s *Server) getReviewStatus(c echo.Context) error {
	wsID := workspaceID(c)
	rid := c.Param("rid")

	session, ok := s.loadReview(wsID, rid)
	if !ok {
		return errorJSON(c, http.StatusNotFound, "review session not found")
	}

	return successJSON(c, session)
}

// submitReview updates a review session with a decision and comments.
func (s *Server) submitReview(c echo.Context) error {
	wsID := workspaceID(c)
	rid := c.Param("rid")

	session, ok := s.loadReview(wsID, rid)
	if !ok {
		return errorJSON(c, http.StatusNotFound, "review session not found")
	}

	var req submitReviewRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	if req.Decision == "" {
		return errorJSON(c, http.StatusBadRequest, "decision is required")
	}

	session.Status = req.Decision
	session.Comment = req.Comment
	session.FileComments = req.FileComments

	s.reviews.Store(reviewKey(wsID, rid), session)

	return successJSON(c, session)
}

func reviewKey(wsID, sessionID string) string {
	return wsID + "/" + sessionID
}

func (s *Server) loadReview(wsID, sessionID string) (*reviewSession, bool) {
	val, ok := s.reviews.Load(reviewKey(wsID, sessionID))
	if !ok {
		return nil, false
	}
	return val.(*reviewSession), true
}

// parseDiffStats parses the summary line from `git diff --stat` output.
// The last line looks like: " 2 files changed, 3 insertions(+), 1 deletion(-)"
func parseDiffStats(statOutput string) *DiffStats {
	stats := &DiffStats{}

	scanner := bufio.NewScanner(strings.NewReader(statOutput))
	var lastLine string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lastLine = line
		}
	}

	if lastLine == "" {
		return stats
	}

	// Parse tokens: "2 files changed, 3 insertions(+), 1 deletion(-)"
	parts := strings.Split(lastLine, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		fields := strings.Fields(part)
		if len(fields) < 2 {
			continue
		}
		n, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		keyword := fields[1]
		switch {
		case strings.HasPrefix(keyword, "file"):
			stats.FilesChanged = n
		case strings.HasPrefix(keyword, "insertion"):
			stats.Insertions = n
		case strings.HasPrefix(keyword, "deletion"):
			stats.Deletions = n
		}
	}

	return stats
}
