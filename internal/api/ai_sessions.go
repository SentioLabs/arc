package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/types"
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
func (s *Server) createAISession(c echo.Context) error {
	var req createAISessionRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request body")
	}

	if req.ID == "" {
		return errorJSON(c, http.StatusBadRequest, "id is required")
	}

	session := &types.AISession{
		ID:             req.ID,
		TranscriptPath: req.TranscriptPath,
		CWD:            req.CWD,
		StartedAt:      time.Now().UTC(),
	}

	ctx := c.Request().Context()
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

// getAISession retrieves an AI session by ID, including its agents.
func (s *Server) getAISession(c echo.Context) error {
	id := c.Param("id")
	ctx := c.Request().Context()

	session, err := s.store.GetAISession(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errorJSON(c, http.StatusNotFound, err.Error())
		}
		return errorJSON(c, http.StatusInternalServerError, err.Error())
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

// listAISessions returns a paginated list of AI sessions.
func (s *Server) listAISessions(c echo.Context) error {
	limit := queryInt(c, "limit", defaultListLimit)
	offset := queryInt(c, "offset", 0)

	sessions, err := s.store.ListAISessions(c.Request().Context(), limit, offset)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return paginatedJSON(c, sessions, len(sessions), limit, offset)
}

// deleteAISession deletes an AI session by ID.
func (s *Server) deleteAISession(c echo.Context) error {
	id := c.Param("id")

	if err := s.store.DeleteAISession(c.Request().Context(), id); err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// createAIAgent creates a new AI agent for a session. If the session does not
// exist, it is auto-created (lazy fallback).
func (s *Server) createAIAgent(c echo.Context) error {
	sessionID := c.Param("id")

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

	agents, err := s.store.ListAIAgents(c.Request().Context(), sessionID)
	if err != nil {
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, agents)
}

// getAIAgent retrieves a single agent by ID.
func (s *Server) getAIAgent(c echo.Context) error {
	agentID := c.Param("aid")
	ctx := c.Request().Context()

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
	ctx := c.Request().Context()

	session, err := s.store.GetAISession(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errorJSON(c, http.StatusNotFound, err.Error())
		}
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	entries, err := readJSONLFile(session.TranscriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errorJSON(c, http.StatusNotFound, "transcript file not found")
		}
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, normalizeTranscriptEntries(entries))
}

// getAgentTranscript derives the agent transcript path from the session
// transcript path: <dir>/<session-id>/subagents/agent-<agent-id>.jsonl
// and returns the entries as a JSON array.
func (s *Server) getAgentTranscript(c echo.Context) error {
	sessionID := c.Param("id")
	agentID := c.Param("aid")
	ctx := c.Request().Context()

	session, err := s.store.GetAISession(ctx, sessionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errorJSON(c, http.StatusNotFound, err.Error())
		}
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	// Derive agent transcript path: <dir>/<session-id>/subagents/agent-<agent-id>.jsonl
	dir := filepath.Dir(session.TranscriptPath)
	agentPath := filepath.Join(dir, sessionID, "subagents", fmt.Sprintf("agent-%s.jsonl", agentID))

	entries, err := readJSONLFile(agentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errorJSON(c, http.StatusNotFound, "agent transcript file not found")
		}
		return errorJSON(c, http.StatusInternalServerError, err.Error())
	}

	return successJSON(c, normalizeTranscriptEntries(entries))
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
	// Claude Code transcripts can have very long lines (tool results, etc.)
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), 10*1024*1024)
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

// normalizeTranscriptEntries flattens Claude Code transcript entries for the
// frontend. Claude Code writes entries as {type, message: {role, content, ...}, ...}
// but the TranscriptViewer expects {role, content, ...} at the top level.
//
// For "user" and "assistant" entries, the message fields are promoted to the top
// level. "progress" entries (tool execution updates) are kept with their type
// preserved so the frontend can render or filter them.
func normalizeTranscriptEntries(raw []json.RawMessage) []json.RawMessage {
	normalized := make([]json.RawMessage, 0, len(raw))
	for _, entry := range raw {
		var outer map[string]json.RawMessage
		if err := json.Unmarshal(entry, &outer); err != nil {
			normalized = append(normalized, entry)
			continue
		}

		msgRaw, hasMessage := outer["message"]
		if !hasMessage {
			normalized = append(normalized, entry)
			continue
		}

		// Determine entry type
		var entryType string
		if t, ok := outer["type"]; ok {
			_ = json.Unmarshal(t, &entryType)
		}

		// Skip progress entries (hook execution metadata, not conversation content)
		if entryType == "progress" {
			continue
		}

		// For user/assistant entries, promote message fields to top level
		if entryType == "user" || entryType == "assistant" {
			var msg map[string]json.RawMessage
			if err := json.Unmarshal(msgRaw, &msg); err != nil {
				normalized = append(normalized, entry)
				continue
			}

			// Start with message fields (role, content, etc.)
			flat := make(map[string]json.RawMessage, len(msg)+2)
			for k, v := range msg {
				flat[k] = v
			}
			// Preserve top-level type and timestamp
			if t, ok := outer["type"]; ok {
				flat["type"] = t
			}
			if ts, ok := outer["timestamp"]; ok {
				flat["timestamp"] = ts
			}

			if out, err := json.Marshal(flat); err == nil {
				normalized = append(normalized, out)
				continue
			}
		}

		normalized = append(normalized, entry)
	}
	return normalized
}
