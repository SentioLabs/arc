package api //nolint:testpackage // tests use internal helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/types"
)

// createTestAISession creates an AI session via the API and returns the response.
func createTestAISession(t *testing.T, e *echo.Echo, id, transcriptPath, cwd string) *types.AISession {
	t.Helper()

	body := fmt.Sprintf(`{"id":%q,"transcript_path":%q,"cwd":%q}`, id, transcriptPath, cwd)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/sessions", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("createAISession returned %d: %s", rec.Code, rec.Body.String())
	}

	var session types.AISession
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil {
		t.Fatalf("failed to parse session response: %v", err)
	}
	return &session
}

// createTestAIAgent creates an AI agent via the API and returns the response.
func createTestAIAgent(t *testing.T, e *echo.Echo, sessionID, agentID string) *types.AIAgent {
	t.Helper()

	body := fmt.Sprintf(`{"id":%q,"description":"test agent","agent_type":"task","model":"opus","status":"completed"}`, agentID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/sessions/"+sessionID+"/agents", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("createAIAgent returned %d: %s", rec.Code, rec.Body.String())
	}

	var agent types.AIAgent
	if err := json.Unmarshal(rec.Body.Bytes(), &agent); err != nil {
		t.Fatalf("failed to parse agent response: %v", err)
	}
	return &agent
}

func TestCreateAISession(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	session := createTestAISession(t, e, "sess-001", "/tmp/transcripts/sess-001.jsonl", "/home/user/project")

	if session.ID != "sess-001" {
		t.Errorf("session.ID = %q, want %q", session.ID, "sess-001")
	}
	if session.TranscriptPath != "/tmp/transcripts/sess-001.jsonl" {
		t.Errorf("session.TranscriptPath = %q, want %q", session.TranscriptPath, "/tmp/transcripts/sess-001.jsonl")
	}
	if session.CWD != "/home/user/project" {
		t.Errorf("session.CWD = %q, want %q", session.CWD, "/home/user/project")
	}
	if session.StartedAt.IsZero() {
		t.Error("session.StartedAt should not be zero")
	}
}

func TestCreateAISession_Idempotent(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	// Create session the first time
	session1 := createTestAISession(t, e, "sess-idem", "/tmp/t/sess-idem.jsonl", "/home/user")

	// Create again with same ID - should return existing, not error
	body := `{"id":"sess-idem","transcript_path":"/tmp/t/sess-idem.jsonl","cwd":"/home/user"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/sessions", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Should succeed (200 or 201)
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("idempotent createAISession returned %d: %s", rec.Code, rec.Body.String())
	}

	var session2 types.AISession
	if err := json.Unmarshal(rec.Body.Bytes(), &session2); err != nil {
		t.Fatalf("failed to parse session response: %v", err)
	}

	if session2.ID != session1.ID {
		t.Errorf("session2.ID = %q, want %q", session2.ID, session1.ID)
	}
}

func TestGetAISession(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	createTestAISession(t, e, "sess-get", "/tmp/t/sess-get.jsonl", "/home/user")
	// Also add an agent so we can verify the response includes agents
	createTestAIAgent(t, e, "sess-get", "agent-001")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/sessions/sess-get", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getAISession returned %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		types.AISession
		Agents []*types.AIAgent `json:"agents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.ID != "sess-get" {
		t.Errorf("resp.ID = %q, want %q", resp.ID, "sess-get")
	}
	if len(resp.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(resp.Agents))
	}
	if resp.Agents[0].ID != "agent-001" {
		t.Errorf("agent.ID = %q, want %q", resp.Agents[0].ID, "agent-001")
	}
}

func TestGetAISession_NotFound(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/sessions/nonexistent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("getAISession for nonexistent returned %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestListAISessions(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	// Create multiple sessions
	for i := range 3 {
		createTestAISession(t, e, fmt.Sprintf("sess-list-%d", i), fmt.Sprintf("/tmp/t/sess-%d.jsonl", i), "/home/user")
	}

	// List with default pagination
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/sessions?limit=2&offset=0", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("listAISessions returned %d: %s", rec.Code, rec.Body.String())
	}

	var resp paginatedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Should have pagination metadata
	if resp.Limit != 2 {
		t.Errorf("resp.Limit = %d, want 2", resp.Limit)
	}
	if resp.Offset != 0 {
		t.Errorf("resp.Offset = %d, want 0", resp.Offset)
	}

	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("failed to marshal data: %v", err)
	}
	var sessions []*types.AISession
	if err := json.Unmarshal(dataBytes, &sessions); err != nil {
		t.Fatalf("failed to parse sessions: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions (limit=2), got %d", len(sessions))
	}
}

func TestDeleteAISession(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	createTestAISession(t, e, "sess-del", "/tmp/t/sess-del.jsonl", "/home/user")

	// Delete
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/ai/sessions/sess-del", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("deleteAISession returned %d: %s", rec.Code, rec.Body.String())
	}

	// Verify deleted
	req = httptest.NewRequest(http.MethodGet, "/api/v1/ai/sessions/sess-del", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("getAISession after delete returned %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestCreateAIAgent(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	createTestAISession(t, e, "sess-agent", "/tmp/t/sess-agent.jsonl", "/home/user")

	agent := createTestAIAgent(t, e, "sess-agent", "agent-002")

	if agent.ID != "agent-002" {
		t.Errorf("agent.ID = %q, want %q", agent.ID, "agent-002")
	}
	if agent.SessionID != "sess-agent" {
		t.Errorf("agent.SessionID = %q, want %q", agent.SessionID, "sess-agent")
	}
	if agent.Status != "completed" {
		t.Errorf("agent.Status = %q, want %q", agent.Status, "completed")
	}
}

func TestCreateAIAgent_LazySession(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	// Do NOT create the session first - the agent endpoint should auto-create it
	body := `{"id":"agent-lazy","description":"lazy test","agent_type":"task","model":"opus","status":"running"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/sessions/sess-lazy/agents", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("createAIAgent (lazy) returned %d: %s", rec.Code, rec.Body.String())
	}

	var agent types.AIAgent
	if err := json.Unmarshal(rec.Body.Bytes(), &agent); err != nil {
		t.Fatalf("failed to parse agent response: %v", err)
	}

	if agent.ID != "agent-lazy" {
		t.Errorf("agent.ID = %q, want %q", agent.ID, "agent-lazy")
	}

	// Verify the session was auto-created
	req = httptest.NewRequest(http.MethodGet, "/api/v1/ai/sessions/sess-lazy", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getAISession (lazy-created) returned %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListAIAgents(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	createTestAISession(t, e, "sess-agents-list", "/tmp/t/sess.jsonl", "/home/user")
	createTestAIAgent(t, e, "sess-agents-list", "agent-a")
	createTestAIAgent(t, e, "sess-agents-list", "agent-b")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/sessions/sess-agents-list/agents", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("listAIAgents returned %d: %s", rec.Code, rec.Body.String())
	}

	var agents []*types.AIAgent
	if err := json.Unmarshal(rec.Body.Bytes(), &agents); err != nil {
		t.Fatalf("failed to parse agents: %v", err)
	}

	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}
}

func TestGetAIAgent(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	createTestAISession(t, e, "sess-get-agent", "/tmp/t/sess.jsonl", "/home/user")
	createTestAIAgent(t, e, "sess-get-agent", "agent-get-001")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/sessions/sess-get-agent/agents/agent-get-001", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getAIAgent returned %d: %s", rec.Code, rec.Body.String())
	}

	var agent types.AIAgent
	if err := json.Unmarshal(rec.Body.Bytes(), &agent); err != nil {
		t.Fatalf("failed to parse agent: %v", err)
	}

	if agent.ID != "agent-get-001" {
		t.Errorf("agent.ID = %q, want %q", agent.ID, "agent-get-001")
	}
}

func TestGetSessionTranscript(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	// Create a temp JSONL file to serve as transcript
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "sess-transcript.jsonl")
	content := `{"type":"message","role":"user","content":"hello"}
{"type":"message","role":"assistant","content":"hi there"}
`
	if err := os.WriteFile(transcriptPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write transcript file: %v", err)
	}

	createTestAISession(t, e, "sess-transcript", transcriptPath, "/home/user")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/sessions/sess-transcript/transcript", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getSessionTranscript returned %d: %s", rec.Code, rec.Body.String())
	}

	var entries []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatalf("failed to parse transcript: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 transcript entries, got %d", len(entries))
	}
	if entries[0]["role"] != "user" {
		t.Errorf("entries[0].role = %q, want %q", entries[0]["role"], "user")
	}
}

func TestGetAgentTranscript(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	// Create directory structure for agent transcript:
	// <dir>/<session-id>/subagents/agent-<agent-id>.jsonl
	tmpDir := t.TempDir()
	transcriptDir := filepath.Join(tmpDir, "sess-at", "subagents")
	if err := os.MkdirAll(transcriptDir, 0o755); err != nil {
		t.Fatalf("failed to create transcript dir: %v", err)
	}

	agentTranscriptPath := filepath.Join(transcriptDir, "agent-agent-at-001.jsonl")
	content := `{"type":"tool_use","name":"bash","input":"ls"}
`
	if err := os.WriteFile(agentTranscriptPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write agent transcript: %v", err)
	}

	// Session transcript_path: <dir>/sess-at.jsonl (the session ID directory is derived)
	sessionTranscriptPath := filepath.Join(tmpDir, "sess-at.jsonl")
	if err := os.WriteFile(sessionTranscriptPath, []byte(`{}`+"\n"), 0o644); err != nil {
		t.Fatalf("failed to write session transcript: %v", err)
	}

	createTestAISession(t, e, "sess-at", sessionTranscriptPath, "/home/user")
	createTestAIAgent(t, e, "sess-at", "agent-at-001")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/sessions/sess-at/agents/agent-at-001/transcript", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getAgentTranscript returned %d: %s", rec.Code, rec.Body.String())
	}

	var entries []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatalf("failed to parse agent transcript: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 transcript entry, got %d", len(entries))
	}
	if entries[0]["name"] != "bash" {
		t.Errorf("entries[0].name = %q, want %q", entries[0]["name"], "bash")
	}
}

func TestGetSessionTranscript_NotFound(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	// Create session with nonexistent transcript path
	createTestAISession(t, e, "sess-nofile", "/nonexistent/path.jsonl", "/home/user")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/sessions/sess-nofile/transcript", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("getSessionTranscript for missing file returned %d, want %d", rec.Code, http.StatusNotFound)
	}
}
