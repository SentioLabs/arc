package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/types"
)

const (
	// maxScannerBuf is the maximum token size for reading JSONL transcript lines.
	// Claude Code transcripts can have very long lines (tool results, etc.).
	maxScannerBuf = 10 * 1024 * 1024 // 10 MiB
)

// createAISessionRequest is the request body for creating an AI session.
type createAISessionRequest struct {
	ID             string `json:"id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
}

// createAIAgentRequest is the request body for creating an AI agent.
type createAIAgentRequest struct {
	ID           string `json:"id"`
	Description  string `json:"description"`
	Prompt       string `json:"prompt"`
	AgentType    string `json:"agent_type"`
	Model        string `json:"model"`
	Status       string `json:"status"`
	DurationMs   *int   `json:"duration_ms"`
	TotalTokens  *int   `json:"total_tokens"`
	ToolUseCount *int   `json:"tool_use_count"`
}

// aiSessionResponse extends AISession with its agents for detail views.
type aiSessionResponse struct {
	types.AISession
	Agents []*types.AIAgent `json:"agents"`
}

// createAISession creates a new AI session. Idempotent: if the session already
// exists, the existing record is returned.
// The projectId is extracted from the URL path and CWD is validated against it.
func (s *Server) createAISession(c echo.Context) error {
	projectID := c.Param("projectId")

	var req createAISessionRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	if req.ID == "" {
		return errorJSON(c, http.StatusBadRequest, "id is required")
	}

	ctx := c.Request().Context()

	// Validate CWD maps to the given project
	if req.CWD != "" {
		ws, err := s.store.ResolveProjectByPath(ctx, req.CWD)
		if err != nil {
			log.Printf("WARN: CWD %q does not resolve to any project: %v", req.CWD, err)
			return errorJSON(c, http.StatusUnprocessableEntity,
				fmt.Sprintf("CWD %q does not resolve to a known project", req.CWD))
		}
		if ws.ProjectID != projectID {
			log.Printf("WARN: CWD %q resolves to project %q, not %q", req.CWD, ws.ProjectID, projectID)
			return errorJSON(c, http.StatusUnprocessableEntity,
				fmt.Sprintf("CWD %q belongs to project %q, not %q", req.CWD, ws.ProjectID, projectID))
		}
	}

	session := &types.AISession{
		ID:             req.ID,
		ProjectID:      projectID,
		TranscriptPath: req.TranscriptPath,
		CWD:            req.CWD,
		StartedAt:      time.Now().UTC(),
	}

	if err := s.store.CreateAISession(ctx, session); err != nil {
		// Check for idempotent case: session already exists
		existing, getErr := s.store.GetAISession(ctx, req.ID)
		if getErr != nil {
			return errorJSON(c, http.StatusInternalServerError, err.Error())
		}
		return successJSON(c, existing)
	}

	return createdJSON(c, session)
}

// validateSessionProject fetches a session and verifies it belongs to the given project.
// Returns the session if valid, or writes an error response and returns nil.
func (s *Server) validateSessionProject(c echo.Context, sessionID, projectID string) (*types.AISession, error) {
	ctx := c.Request().Context()
	session, err := s.store.GetAISession(ctx, sessionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, errorJSON(c, http.StatusNotFound, err.Error())
		}
		return nil, errorJSON(c, http.StatusInternalServerError, err.Error())
	}
	if session.ProjectID != projectID {
		msg := fmt.Sprintf("session %q not found in project %q", sessionID, projectID)
		return nil, errorJSON(c, http.StatusNotFound, msg)
	}
	return session, nil
}

// getAISession retrieves an AI session by ID, including its agents.
func (s *Server) getAISession(c echo.Context) error {
	id := c.Param("id")
	projectID := c.Param("projectId")
	ctx := c.Request().Context()

	session, err := s.validateSessionProject(c, id, projectID)
	if err != nil {
		return err
	}

	agents, err := s.store.ListAIAgents(ctx, id)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, aiSessionResponse{
		AISession: *session,
		Agents:    agents,
	})
}

// listAISessionsByProject returns a paginated list of AI sessions for a project.
func (s *Server) listAISessionsByProject(c echo.Context) error {
	projectID := c.Param("projectId")
	limit := queryInt(c, "limit", defaultListLimit)
	offset := queryInt(c, "offset", 0)
	ctx := c.Request().Context()

	sessions, err := s.store.ListAISessionsByProject(ctx, projectID, limit, offset)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	total, err := s.store.CountAISessionsByProject(ctx, projectID)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return paginatedJSON(c, sessions, int(total), limit, offset)
}

// deleteAISession deletes an AI session by ID.
func (s *Server) deleteAISession(c echo.Context) error {
	id := c.Param("id")
	projectID := c.Param("projectId")

	if _, err := s.validateSessionProject(c, id, projectID); err != nil {
		return err
	}

	if err := s.store.DeleteAISession(c.Request().Context(), id); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// batchDeleteAISessionsRequest is the request body for batch-deleting AI sessions.
type batchDeleteAISessionsRequest struct {
	IDs []string `json:"ids"`
}

// batchDeleteAISessions deletes multiple AI sessions by ID.
// Only sessions belonging to the path projectId are deleted.
func (s *Server) batchDeleteAISessions(c echo.Context) error {
	projectID := c.Param("projectId")

	var req batchDeleteAISessionsRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	if len(req.IDs) == 0 {
		return errorJSON(c, http.StatusBadRequest, "ids is required")
	}

	ctx := c.Request().Context()
	deleted := 0
	for _, id := range req.IDs {
		// Verify session belongs to this project before deleting
		session, err := s.store.GetAISession(ctx, id)
		if err != nil {
			continue
		}
		if session.ProjectID != projectID {
			continue
		}
		if err := s.store.DeleteAISession(ctx, id); err == nil {
			deleted++
		}
	}

	return successJSON(c, map[string]int{"deleted": deleted})
}

// createAIAgent creates a new AI agent for a session. If the session does not
// exist, it is auto-created (lazy fallback).
func (s *Server) createAIAgent(c echo.Context) error {
	sessionID := c.Param("id")
	projectID := c.Param("projectId")

	var req createAIAgentRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	if req.ID == "" {
		return errorJSON(c, http.StatusBadRequest, "id is required")
	}

	ctx := c.Request().Context()

	// Lazy session creation: if session doesn't exist, auto-create it
	if _, err := s.store.GetAISession(ctx, sessionID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			lazySession := &types.AISession{
				ID:        sessionID,
				ProjectID: projectID,
				StartedAt: time.Now().UTC(),
			}
			if createErr := s.store.CreateAISession(ctx, lazySession); createErr != nil {
				return errorJSON(c, http.StatusInternalServerError, createErr.Error())
			}
		} else {
			return errorJSON(c, http.StatusInternalServerError, err.Error())
		}
	}

	agent := &types.AIAgent{
		ID:           req.ID,
		SessionID:    sessionID,
		Description:  req.Description,
		Prompt:       req.Prompt,
		AgentType:    req.AgentType,
		Model:        req.Model,
		Status:       req.Status,
		DurationMs:   req.DurationMs,
		TotalTokens:  req.TotalTokens,
		ToolUseCount: req.ToolUseCount,
		CreatedAt:    time.Now().UTC(),
	}

	if err := s.store.CreateAIAgent(ctx, agent); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return createdJSON(c, agent)
}

// listAIAgents returns all agents for a session.
func (s *Server) listAIAgents(c echo.Context) error {
	sessionID := c.Param("id")
	projectID := c.Param("projectId")

	if _, err := s.validateSessionProject(c, sessionID, projectID); err != nil {
		return err
	}

	agents, err := s.store.ListAIAgents(c.Request().Context(), sessionID)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, agents)
}

// getAIAgent retrieves a single agent by ID.
func (s *Server) getAIAgent(c echo.Context) error {
	sessionID := c.Param("id")
	projectID := c.Param("projectId")
	agentID := c.Param("aid")
	ctx := c.Request().Context()

	if _, err := s.validateSessionProject(c, sessionID, projectID); err != nil {
		return err
	}

	agent, err := s.store.GetAIAgent(ctx, agentID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errorJSON(c, http.StatusNotFound, err.Error())
		}
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, agent)
}

// getSessionTranscript reads the session's transcript JSONL file from disk
// and returns the entries as a JSON array.
func (s *Server) getSessionTranscript(c echo.Context) error {
	id := c.Param("id")
	projectID := c.Param("projectId")

	session, err := s.validateSessionProject(c, id, projectID)
	if err != nil {
		return err
	}

	entries, readErr := readJSONLFile(session.TranscriptPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return errorJSON(c, http.StatusNotFound, "transcript file not found")
		}
		return errorJSON(c, http.StatusInternalServerError, readErr.Error())
	}

	return successJSON(c, NormalizeTranscriptEntries(entries))
}

// getAgentTranscript derives the agent transcript path from the session
// transcript path: <dir>/<session-id>/subagents/agent-<agent-id>.jsonl
// and returns the entries as a JSON array.
func (s *Server) getAgentTranscript(c echo.Context) error {
	sessionID := c.Param("id")
	projectID := c.Param("projectId")
	agentID := c.Param("aid")

	session, err := s.validateSessionProject(c, sessionID, projectID)
	if err != nil {
		return err
	}

	// Derive agent transcript path: <dir>/<session-id>/subagents/agent-<agent-id>.jsonl
	dir := filepath.Dir(session.TranscriptPath)
	agentPath := filepath.Join(dir, sessionID, "subagents", fmt.Sprintf("agent-%s.jsonl", agentID))

	entries, readErr := readJSONLFile(agentPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return errorJSON(c, http.StatusNotFound, "agent transcript file not found")
		}
		return errorJSON(c, http.StatusInternalServerError, readErr.Error())
	}

	return successJSON(c, NormalizeTranscriptEntries(entries))
}

// readJSONLFile reads a JSONL file line by line and returns the entries as a
// slice of json.RawMessage values. Empty lines are skipped. Returns
// os.ErrNotExist if the file does not exist on disk.
func readJSONLFile(path string) ([]json.RawMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []json.RawMessage
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), maxScannerBuf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		entries = append(entries, json.RawMessage(line))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading jsonl file: %w", err)
	}

	return entries, nil
}

// NormalizeTranscriptEntries flattens Claude Code transcript entries for the
// frontend. Claude Code writes entries as {type, message: {role, content, ...}, ...}
// but the TranscriptViewer expects {role, content, ...} at the top level.
//
// For "user" and "assistant" entries, the message fields are promoted to the top
// level. "progress" entries (tool execution updates) are kept with their type
// preserved so the frontend can render or filter them.
func NormalizeTranscriptEntries(raw []json.RawMessage) []json.RawMessage {
	normalized := make([]json.RawMessage, 0, len(raw))
	for _, entry := range raw {
		if out, keep := normalizeEntry(entry); keep {
			normalized = append(normalized, out)
		}
	}
	return normalized
}

// normalizeEntry processes a single transcript entry. It returns the (possibly
// transformed) entry and whether it should be kept in the output.
func normalizeEntry(entry json.RawMessage) (json.RawMessage, bool) {
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(entry, &outer); err != nil {
		return entry, true
	}

	entryType := jsonStringField(outer, "type")

	// Skip progress entries (hook execution metadata, not conversation content)
	if entryType == "progress" {
		return nil, false
	}

	msgRaw, hasMessage := outer["message"]
	if !hasMessage {
		return entry, true
	}

	// For user/assistant entries, promote message fields to top level
	if entryType == "user" || entryType == "assistant" {
		if flat, err := flattenMessage(outer, msgRaw); err == nil {
			return flat, true
		}
	}

	return entry, true
}

// flattenMessage promotes message fields to the top level, preserving type and timestamp.
func flattenMessage(outer map[string]json.RawMessage, msgRaw json.RawMessage) (json.RawMessage, error) {
	var msg map[string]json.RawMessage
	if err := json.Unmarshal(msgRaw, &msg); err != nil {
		return nil, err
	}

	flat := make(map[string]json.RawMessage, len(msg)+len(outer))
	for k, v := range msg {
		flat[k] = v
	}
	// Preserve top-level type and timestamp
	for _, key := range []string{"type", "timestamp"} {
		if v, ok := outer[key]; ok {
			flat[key] = v
		}
	}

	return json.Marshal(flat)
}

// jsonStringField extracts a string value from a JSON object map.
func jsonStringField(m map[string]json.RawMessage, key string) string {
	raw, ok := m[key]
	if !ok {
		return ""
	}
	var s string
	_ = json.Unmarshal(raw, &s)
	return s
}
