package api //nolint:testpackage // tests use internal helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/sentiolabs/arc/internal/types"
)

const (
	otherProjectBody = `{"name":"Other Project","prefix":"otp"}`
)

// aiSessionOpts holds parameters for creating an AI session in tests.
type aiSessionOpts struct {
	ProjectID      string
	ID             string
	TranscriptPath string
	CWD            string
}

// testProjectID returns the ID of a project created via the API.
func testProjectID(t *testing.T, e *echo.Echo) string {
	t.Helper()

	body := `{"name":"AI Test Project","prefix":"aitp"}`
	req := httptest.NewRequest(
		http.MethodPost, "/api/v1/projects",
		bytes.NewBufferString(body),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create project: %s", rec.Body.String())
	}

	var p types.Project
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("failed to parse project response: %v", err)
	}
	return p.ID
}

// addWorkspaceToProject adds a workspace path to a project.
func addWorkspaceToProject(
	t *testing.T, e *echo.Echo, projectID, path string,
) {
	t.Helper()

	body := fmt.Sprintf(`{"path":%q}`, path)
	wsURL := "/api/v1/projects/" + projectID + "/workspaces"
	req := httptest.NewRequest(
		http.MethodPost, wsURL, bytes.NewBufferString(body),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to add workspace: %s", rec.Body.String())
	}
}

// createTestAISession creates an AI session via the project-scoped API.
func createTestAISession(
	t *testing.T, e *echo.Echo, opts aiSessionOpts,
) *types.AISession {
	t.Helper()

	body := fmt.Sprintf(
		`{"id":%q,"transcript_path":%q,"cwd":%q}`,
		opts.ID, opts.TranscriptPath, opts.CWD,
	)
	sessURL := fmt.Sprintf(
		"/api/v1/projects/%s/ai/sessions", opts.ProjectID,
	)
	req := httptest.NewRequest(
		http.MethodPost, sessURL, bytes.NewBufferString(body),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("createAISession returned %d: %s",
			rec.Code, rec.Body.String())
	}

	var session types.AISession
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil {
		t.Fatalf("failed to parse session response: %v", err)
	}
	return &session
}

// aiAgentOpts holds parameters for creating an AI agent in tests.
type aiAgentOpts struct {
	ProjectID string
	SessionID string
	AgentID   string
	Status    string
}

// createTestAIAgentWithOpts creates an AI agent with the given options.
func createTestAIAgentWithOpts(
	t *testing.T, e *echo.Echo, opts aiAgentOpts,
) *types.AIAgent {
	t.Helper()

	status := opts.Status
	if status == "" {
		status = "completed"
	}

	body := fmt.Sprintf(
		`{"id":%q,"description":"test agent",`+
			`"agent_type":"task","model":"opus","status":%q}`,
		opts.AgentID, status,
	)
	agentURL := fmt.Sprintf(
		"/api/v1/projects/%s/ai/sessions/%s/agents",
		opts.ProjectID, opts.SessionID,
	)
	req := httptest.NewRequest(
		http.MethodPost, agentURL, bytes.NewBufferString(body),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("createAIAgentWithOpts returned %d: %s",
			rec.Code, rec.Body.String())
	}

	var agent types.AIAgent
	if err := json.Unmarshal(rec.Body.Bytes(), &agent); err != nil {
		t.Fatalf("failed to parse agent response: %v", err)
	}
	return &agent
}

// createTestAIAgent creates an AI agent via the project-scoped API.
func createTestAIAgent(
	t *testing.T, e *echo.Echo,
	projectID, sessionID, agentID string,
) *types.AIAgent {
	t.Helper()

	body := fmt.Sprintf(
		`{"id":%q,"description":"test agent",`+
			`"agent_type":"task","model":"opus","status":"completed"}`,
		agentID,
	)
	agentURL := fmt.Sprintf(
		"/api/v1/projects/%s/ai/sessions/%s/agents",
		projectID, sessionID,
	)
	req := httptest.NewRequest(
		http.MethodPost, agentURL, bytes.NewBufferString(body),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("createAIAgent returned %d: %s",
			rec.Code, rec.Body.String())
	}

	var agent types.AIAgent
	if err := json.Unmarshal(rec.Body.Bytes(), &agent); err != nil {
		t.Fatalf("failed to parse agent response: %v", err)
	}
	return &agent
}

// sessionURL builds a project-scoped AI session URL.
func sessionURL(projectID, path string) string {
	return fmt.Sprintf(
		"/api/v1/projects/%s/ai/sessions%s", projectID, path,
	)
}

func TestCreateAISession(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectID := testProjectID(t, e)
	cwd := testUserProjectPath
	addWorkspaceToProject(t, e, projectID, cwd)

	session := createTestAISession(t, e, aiSessionOpts{
		ProjectID:      projectID,
		ID:             "sess-001",
		TranscriptPath: "/tmp/transcripts/sess-001.jsonl",
		CWD:            cwd,
	})

	if session.ID != "sess-001" {
		t.Errorf("session.ID = %q, want %q", session.ID, "sess-001")
	}
	if session.ProjectID != projectID {
		t.Errorf("session.ProjectID = %q, want %q",
			session.ProjectID, projectID)
	}
	if session.TranscriptPath != "/tmp/transcripts/sess-001.jsonl" {
		t.Errorf("session.TranscriptPath = %q, want %q",
			session.TranscriptPath, "/tmp/transcripts/sess-001.jsonl")
	}
	if session.CWD != cwd {
		t.Errorf("session.CWD = %q, want %q", session.CWD, cwd)
	}
	if session.StartedAt.IsZero() {
		t.Error("session.StartedAt should not be zero")
	}
}

func TestCreateAISession_CWDMismatch(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectID := testProjectID(t, e)
	addWorkspaceToProject(t, e, projectID, testUserProjectPath)

	// Use a CWD that does not map to any project
	body := `{"id":"sess-bad","transcript_path":"/tmp/t.jsonl",` +
		`"cwd":"/some/other/path"}`
	req := httptest.NewRequest(
		http.MethodPost, sessionURL(projectID, ""),
		bytes.NewBufferString(body),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for CWD mismatch, got %d: %s",
			rec.Code, rec.Body.String())
	}
}

func TestCreateAISession_CWDWrongProject(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	// Create two projects with different workspaces
	projectA := testProjectID(t, e)
	addWorkspaceToProject(t, e, projectA, "/home/user/project-a")

	// Create second project
	req := httptest.NewRequest(
		http.MethodPost, "/api/v1/projects",
		bytes.NewBufferString(otherProjectBody),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var projB types.Project
	_ = json.Unmarshal(rec.Body.Bytes(), &projB)
	addWorkspaceToProject(t, e, projB.ID, "/home/user/project-b")

	// Try to create session on project A with CWD from project B
	body := `{"id":"sess-wrong","transcript_path":"/tmp/t.jsonl",` +
		`"cwd":"/home/user/project-b"}`
	req = httptest.NewRequest(
		http.MethodPost, sessionURL(projectA, ""),
		bytes.NewBufferString(body),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for wrong project CWD, got %d: %s",
			rec.Code, rec.Body.String())
	}
}

func TestCreateAISession_Idempotent(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectID := testProjectID(t, e)
	cwd := testUserProjectPath
	addWorkspaceToProject(t, e, projectID, cwd)

	session1 := createTestAISession(t, e, aiSessionOpts{
		ProjectID:      projectID,
		ID:             "sess-idem",
		TranscriptPath: "/tmp/t/sess-idem.jsonl",
		CWD:            cwd,
	})

	// Create again with same ID - should return existing
	body := fmt.Sprintf(
		`{"id":"sess-idem","transcript_path":"/tmp/t/sess-idem.jsonl","cwd":%q}`,
		cwd,
	)
	req := httptest.NewRequest(
		http.MethodPost, sessionURL(projectID, ""),
		bytes.NewBufferString(body),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("idempotent createAISession returned %d: %s",
			rec.Code, rec.Body.String())
	}

	var session2 types.AISession
	if err := json.Unmarshal(rec.Body.Bytes(), &session2); err != nil {
		t.Fatalf("failed to parse session response: %v", err)
	}

	if session2.ID != session1.ID {
		t.Errorf("session2.ID = %q, want %q",
			session2.ID, session1.ID)
	}
}

func TestGetAISession(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectID := testProjectID(t, e)
	cwd := testUserProjectPath
	addWorkspaceToProject(t, e, projectID, cwd)

	createTestAISession(t, e, aiSessionOpts{
		ProjectID:      projectID,
		ID:             "sess-get",
		TranscriptPath: "/tmp/t/sess-get.jsonl",
		CWD:            cwd,
	})
	createTestAIAgent(t, e, projectID, "sess-get", "agent-001")

	req := httptest.NewRequest(
		http.MethodGet, sessionURL(projectID, "/sess-get"), nil,
	)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getAISession returned %d: %s",
			rec.Code, rec.Body.String())
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
		t.Errorf("agent.ID = %q, want %q",
			resp.Agents[0].ID, "agent-001")
	}
}

func TestGetAISession_NotFound(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectID := testProjectID(t, e)

	req := httptest.NewRequest(
		http.MethodGet,
		sessionURL(projectID, "/nonexistent"), nil,
	)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("getAISession for nonexistent returned %d, want %d",
			rec.Code, http.StatusNotFound)
	}
}

func TestGetAISession_ProjectMismatch(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectA := testProjectID(t, e)
	cwd := testUserProjectPath
	addWorkspaceToProject(t, e, projectA, cwd)

	createTestAISession(t, e, aiSessionOpts{
		ProjectID:      projectA,
		ID:             "sess-mismatch",
		TranscriptPath: "/tmp/t/sess.jsonl",
		CWD:            cwd,
	})

	// Create another project
	req := httptest.NewRequest(
		http.MethodPost, "/api/v1/projects",
		bytes.NewBufferString(otherProjectBody),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var projB types.Project
	_ = json.Unmarshal(rec.Body.Bytes(), &projB)

	// Try to get session from wrong project
	req = httptest.NewRequest(
		http.MethodGet,
		sessionURL(projB.ID, "/sess-mismatch"), nil,
	)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("getAISession with wrong project returned %d, want %d",
			rec.Code, http.StatusNotFound)
	}
}

func TestListAISessionsByProject(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectID := testProjectID(t, e)
	cwd := testUserProjectPath
	addWorkspaceToProject(t, e, projectID, cwd)

	// Create multiple sessions
	for i := range 3 {
		createTestAISession(t, e, aiSessionOpts{
			ProjectID:      projectID,
			ID:             fmt.Sprintf("sess-list-%d", i),
			TranscriptPath: fmt.Sprintf("/tmp/t/sess-%d.jsonl", i),
			CWD:            cwd,
		})
	}

	// List with pagination
	req := httptest.NewRequest(
		http.MethodGet,
		sessionURL(projectID, "?limit=2&offset=0"), nil,
	)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("listAISessionsByProject returned %d: %s",
			rec.Code, rec.Body.String())
	}

	var resp paginatedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

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
		t.Errorf("expected 2 sessions (limit=2), got %d",
			len(sessions))
	}
}

func TestDeleteAISession(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectID := testProjectID(t, e)
	cwd := testUserProjectPath
	addWorkspaceToProject(t, e, projectID, cwd)

	createTestAISession(t, e, aiSessionOpts{
		ProjectID:      projectID,
		ID:             "sess-del",
		TranscriptPath: "/tmp/t/sess-del.jsonl",
		CWD:            cwd,
	})

	// Delete
	delURL := sessionURL(projectID, "/sess-del")
	req := httptest.NewRequest(http.MethodDelete, delURL, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("deleteAISession returned %d: %s",
			rec.Code, rec.Body.String())
	}

	// Verify deleted
	req = httptest.NewRequest(http.MethodGet, delURL, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("getAISession after delete returned %d, want %d",
			rec.Code, http.StatusNotFound)
	}
}

func TestDeleteAISession_ProjectMismatch(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectA := testProjectID(t, e)
	cwd := testUserProjectPath
	addWorkspaceToProject(t, e, projectA, cwd)

	createTestAISession(t, e, aiSessionOpts{
		ProjectID:      projectA,
		ID:             "sess-del-mis",
		TranscriptPath: "/tmp/t/sess.jsonl",
		CWD:            cwd,
	})

	// Create another project
	req := httptest.NewRequest(
		http.MethodPost, "/api/v1/projects",
		bytes.NewBufferString(otherProjectBody),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var projB types.Project
	_ = json.Unmarshal(rec.Body.Bytes(), &projB)

	// Try to delete session from wrong project
	req = httptest.NewRequest(
		http.MethodDelete,
		sessionURL(projB.ID, "/sess-del-mis"), nil,
	)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("deleteAISession wrong project returned %d, want %d",
			rec.Code, http.StatusNotFound)
	}
}

func TestBatchDeleteAISessions(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectID := testProjectID(t, e)
	cwd := testUserProjectPath
	addWorkspaceToProject(t, e, projectID, cwd)

	createTestAISession(t, e, aiSessionOpts{
		ProjectID: projectID, ID: "sess-bd-1",
		TranscriptPath: "/tmp/t/1.jsonl", CWD: cwd,
	})
	createTestAISession(t, e, aiSessionOpts{
		ProjectID: projectID, ID: "sess-bd-2",
		TranscriptPath: "/tmp/t/2.jsonl", CWD: cwd,
	})

	body := `{"ids":["sess-bd-1","sess-bd-2"]}`
	req := httptest.NewRequest(
		http.MethodPost,
		sessionURL(projectID, "/batch-delete"),
		bytes.NewBufferString(body),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("batchDeleteAISessions returned %d: %s",
			rec.Code, rec.Body.String())
	}

	var result map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if result["deleted"] != 2 {
		t.Errorf("expected 2 deleted, got %d", result["deleted"])
	}
}

func TestCreateAIAgent(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectID := testProjectID(t, e)
	cwd := testUserProjectPath
	addWorkspaceToProject(t, e, projectID, cwd)

	createTestAISession(t, e, aiSessionOpts{
		ProjectID:      projectID,
		ID:             "sess-agent",
		TranscriptPath: "/tmp/t/sess-agent.jsonl",
		CWD:            cwd,
	})

	agent := createTestAIAgent(t, e, projectID, "sess-agent", "agent-002")

	if agent.ID != "agent-002" {
		t.Errorf("agent.ID = %q, want %q", agent.ID, "agent-002")
	}
	if agent.SessionID != "sess-agent" {
		t.Errorf("agent.SessionID = %q, want %q",
			agent.SessionID, "sess-agent")
	}
	if agent.Status != "completed" {
		t.Errorf("agent.Status = %q, want %q",
			agent.Status, "completed")
	}
}

func TestCreateAIAgent_LazySession(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectID := testProjectID(t, e)

	// Do NOT create the session first - auto-create via agent endpoint
	body := `{"id":"agent-lazy","description":"lazy test",` +
		`"agent_type":"task","model":"opus","status":"running"}`
	agentURL := fmt.Sprintf(
		"/api/v1/projects/%s/ai/sessions/sess-lazy/agents",
		projectID,
	)
	req := httptest.NewRequest(
		http.MethodPost, agentURL, bytes.NewBufferString(body),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("createAIAgent (lazy) returned %d: %s",
			rec.Code, rec.Body.String())
	}

	var agent types.AIAgent
	if err := json.Unmarshal(rec.Body.Bytes(), &agent); err != nil {
		t.Fatalf("failed to parse agent response: %v", err)
	}

	if agent.ID != "agent-lazy" {
		t.Errorf("agent.ID = %q, want %q", agent.ID, "agent-lazy")
	}

	// Verify the session was auto-created
	req = httptest.NewRequest(
		http.MethodGet,
		sessionURL(projectID, "/sess-lazy"), nil,
	)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getAISession (lazy-created) returned %d: %s",
			rec.Code, rec.Body.String())
	}
}

func TestListAIAgents(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectID := testProjectID(t, e)
	cwd := testUserProjectPath
	addWorkspaceToProject(t, e, projectID, cwd)

	createTestAISession(t, e, aiSessionOpts{
		ProjectID:      projectID,
		ID:             "sess-agents-list",
		TranscriptPath: "/tmp/t/sess.jsonl",
		CWD:            cwd,
	})
	createTestAIAgent(t, e, projectID, "sess-agents-list", "agent-a")
	createTestAIAgent(t, e, projectID, "sess-agents-list", "agent-b")

	agentsURL := fmt.Sprintf(
		"/api/v1/projects/%s/ai/sessions/sess-agents-list/agents",
		projectID,
	)
	req := httptest.NewRequest(http.MethodGet, agentsURL, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("listAIAgents returned %d: %s",
			rec.Code, rec.Body.String())
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

	projectID := testProjectID(t, e)
	cwd := testUserProjectPath
	addWorkspaceToProject(t, e, projectID, cwd)

	createTestAISession(t, e, aiSessionOpts{
		ProjectID:      projectID,
		ID:             "sess-get-agent",
		TranscriptPath: "/tmp/t/sess.jsonl",
		CWD:            cwd,
	})
	createTestAIAgent(t, e, projectID, "sess-get-agent", "agent-get-001")

	agentURL := fmt.Sprintf(
		"/api/v1/projects/%s/ai/sessions/sess-get-agent/agents/agent-get-001",
		projectID,
	)
	req := httptest.NewRequest(http.MethodGet, agentURL, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getAIAgent returned %d: %s",
			rec.Code, rec.Body.String())
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

	projectID := testProjectID(t, e)
	cwd := testUserProjectPath
	addWorkspaceToProject(t, e, projectID, cwd)

	// Create a temp JSONL file to serve as transcript
	tmpDir := t.TempDir()
	tPath := filepath.Join(tmpDir, "sess-transcript.jsonl")
	content := "{\"type\":\"message\",\"role\":\"user\"," +
		"\"content\":\"hello\"}\n" +
		"{\"type\":\"message\",\"role\":\"assistant\"," +
		"\"content\":\"hi there\"}\n"
	if err := os.WriteFile(tPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write transcript file: %v", err)
	}

	createTestAISession(t, e, aiSessionOpts{
		ProjectID:      projectID,
		ID:             "sess-transcript",
		TranscriptPath: tPath,
		CWD:            cwd,
	})

	req := httptest.NewRequest(
		http.MethodGet,
		sessionURL(projectID, "/sess-transcript/transcript"), nil,
	)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getSessionTranscript returned %d: %s",
			rec.Code, rec.Body.String())
	}

	var entries []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatalf("failed to parse transcript: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 transcript entries, got %d",
			len(entries))
	}
	if entries[0]["role"] != "user" {
		t.Errorf("entries[0].role = %q, want %q",
			entries[0]["role"], "user")
	}
}

func TestGetAgentTranscript(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectID := testProjectID(t, e)
	cwd := testUserProjectPath
	addWorkspaceToProject(t, e, projectID, cwd)

	// Create directory structure for agent transcript
	tmpDir := t.TempDir()
	transcriptDir := filepath.Join(
		tmpDir, "sess-at", "subagents",
	)
	if err := os.MkdirAll(transcriptDir, 0o755); err != nil {
		t.Fatalf("failed to create transcript dir: %v", err)
	}

	agentFile := filepath.Join(
		transcriptDir, "agent-agent-at-001.jsonl",
	)
	content := `{"type":"tool_use","name":"bash","input":"ls"}` + "\n"
	if err := os.WriteFile(agentFile, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write agent transcript: %v", err)
	}

	sessFile := filepath.Join(tmpDir, "sess-at.jsonl")
	if err := os.WriteFile(sessFile, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("failed to write session transcript: %v", err)
	}

	createTestAISession(t, e, aiSessionOpts{
		ProjectID:      projectID,
		ID:             "sess-at",
		TranscriptPath: sessFile,
		CWD:            cwd,
	})
	createTestAIAgent(t, e, projectID, "sess-at", "agent-at-001")

	agentTURL := fmt.Sprintf(
		"/api/v1/projects/%s/ai/sessions/sess-at"+
			"/agents/agent-at-001/transcript",
		projectID,
	)
	req := httptest.NewRequest(http.MethodGet, agentTURL, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("getAgentTranscript returned %d: %s",
			rec.Code, rec.Body.String())
	}

	var entries []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatalf("failed to parse agent transcript: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 transcript entry, got %d",
			len(entries))
	}
	if entries[0]["name"] != "bash" {
		t.Errorf("entries[0].name = %q, want %q",
			entries[0]["name"], "bash")
	}
}

func TestGetSessionTranscript_NotFound(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectID := testProjectID(t, e)
	cwd := testUserProjectPath
	addWorkspaceToProject(t, e, projectID, cwd)

	createTestAISession(t, e, aiSessionOpts{
		ProjectID:      projectID,
		ID:             "sess-nofile",
		TranscriptPath: "/nonexistent/path.jsonl",
		CWD:            cwd,
	})

	req := httptest.NewRequest(
		http.MethodGet,
		sessionURL(projectID, "/sess-nofile/transcript"), nil,
	)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("getSessionTranscript for missing file returned %d, want %d",
			rec.Code, http.StatusNotFound)
	}
}

func TestListAISessionsWithSummary(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projectID := testProjectID(t, e)
	cwd := testUserProjectPath
	addWorkspaceToProject(t, e, projectID, cwd)

	// Create 2 sessions
	createTestAISession(t, e, aiSessionOpts{
		ProjectID:      projectID,
		ID:             "sess-sum-1",
		TranscriptPath: "/tmp/t/sess-sum-1.jsonl",
		CWD:            cwd,
	})
	createTestAISession(t, e, aiSessionOpts{
		ProjectID:      projectID,
		ID:             "sess-sum-2",
		TranscriptPath: "/tmp/t/sess-sum-2.jsonl",
		CWD:            cwd,
	})

	// Add agents with mixed statuses to sess-sum-1
	createTestAIAgentWithOpts(t, e, aiAgentOpts{
		ProjectID: projectID, SessionID: "sess-sum-1",
		AgentID: "ag-1", Status: "running",
	})
	createTestAIAgentWithOpts(t, e, aiAgentOpts{
		ProjectID: projectID, SessionID: "sess-sum-1",
		AgentID: "ag-2", Status: "completed",
	})
	createTestAIAgentWithOpts(t, e, aiAgentOpts{
		ProjectID: projectID, SessionID: "sess-sum-1",
		AgentID: "ag-3", Status: "error",
	})
	createTestAIAgentWithOpts(t, e, aiAgentOpts{
		ProjectID: projectID, SessionID: "sess-sum-1",
		AgentID: "ag-4", Status: "completed",
	})

	// sess-sum-2 has no agents — agent_summary should be omitted

	// GET list
	req := httptest.NewRequest(
		http.MethodGet,
		sessionURL(projectID, ""), nil,
	)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("listAISessionsByProject returned %d: %s",
			rec.Code, rec.Body.String())
	}

	var resp paginatedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("failed to marshal data: %v", err)
	}

	// Parse as raw JSON array to inspect agent_summary
	var items []json.RawMessage
	if err := json.Unmarshal(dataBytes, &items); err != nil {
		t.Fatalf("failed to parse items: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(items))
	}

	// Build a map of session ID -> parsed item
	type listItem struct {
		ID           string              `json:"id"`
		AgentSummary *types.AgentSummary `json:"agent_summary"`
	}

	parsed := make(map[string]*listItem)
	for _, raw := range items {
		var item listItem
		if err := json.Unmarshal(raw, &item); err != nil {
			t.Fatalf("failed to parse list item: %v", err)
		}
		parsed[item.ID] = &item
	}

	// sess-sum-1: should have agent_summary with correct counts
	s1 := parsed["sess-sum-1"]
	if s1 == nil {
		t.Fatal("sess-sum-1 not found in response")
	}
	if s1.AgentSummary == nil {
		t.Fatal("sess-sum-1 agent_summary should not be nil")
	}
	if s1.AgentSummary.AgentCount != 4 {
		t.Errorf("agent_count = %d, want 4", s1.AgentSummary.AgentCount)
	}
	if s1.AgentSummary.RunningCount != 1 {
		t.Errorf("running_count = %d, want 1", s1.AgentSummary.RunningCount)
	}
	if s1.AgentSummary.CompletedCount != 2 {
		t.Errorf("completed_count = %d, want 2", s1.AgentSummary.CompletedCount)
	}
	if s1.AgentSummary.ErrorCount != 1 {
		t.Errorf("error_count = %d, want 1", s1.AgentSummary.ErrorCount)
	}

	// sess-sum-2: should have no agent_summary (omitted)
	s2 := parsed["sess-sum-2"]
	if s2 == nil {
		t.Fatal("sess-sum-2 not found in response")
	}
	if s2.AgentSummary != nil {
		t.Errorf("sess-sum-2 agent_summary should be nil, got %+v",
			s2.AgentSummary)
	}
}

// TestCreateAISession_WorktreePathUnderRegistered verifies that a CWD that is a
// subdirectory of a registered workspace path resolves correctly (prefix match).
func TestCreateAISession_WorktreePathUnderRegistered(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projID := createNamedProject(t, e, "worktree-prefix-proj", "wtp")
	addWorkspaceToProject(t, e, projID, "/repos/main")

	body := strings.NewReader(`{"id":"sess-wt-1","cwd":"/repos/main/.worktrees/feature-x"}`)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/projects/"+projID+"/ai/sessions",
		body,
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d. body=%s",
			rec.Code, http.StatusCreated, rec.Body.String())
	}
}

// TestCreateAISession_LinkedWorktreeOutsideRegistered verifies that a CWD inside
// a linked git worktree (outside the registered prefix) resolves via DetectMainRepo.
func TestCreateAISession_LinkedWorktreeOutsideRegistered(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	tmp := t.TempDir()
	mainDir := filepath.Join(tmp, "main")
	wtDir := filepath.Join(tmp, "feature-x")

	// Real git init + worktree add so DetectMainRepo can follow the .git pointer.
	mustRun(t, mainDir, "git", "init", "-q")
	mustRun(t, mainDir, "git", "commit", "--allow-empty", "-m", "init", "-q")
	mustRun(t, mainDir, "git", "worktree", "add", "-q", "-b", "feature-x", wtDir)
	t.Cleanup(func() {
		_ = exec.Command("git", "worktree", "remove", "--force", wtDir).Run()
	})

	projID := createNamedProject(t, e, "linked-worktree-proj", "lwp")
	addWorkspaceToProject(t, e, projID, mainDir)

	body := strings.NewReader(fmt.Sprintf(`{"id":"sess-wt-2","cwd":%q}`, wtDir))
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/projects/"+projID+"/ai/sessions",
		body,
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d. body=%s",
			rec.Code, http.StatusCreated, rec.Body.String())
	}
}

// TestCreateAISession_RejectCrossProject verifies that posting to project A with
// a CWD registered under project B is still rejected with 422.
func TestCreateAISession_RejectCrossProject(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()
	e := server.echo

	projAID := createNamedProject(t, e, "cross-proj-a", "cpa")
	projBID := createNamedProject(t, e, "cross-proj-b", "cpb")
	addWorkspaceToProject(t, e, projAID, "/repos/a")
	addWorkspaceToProject(t, e, projBID, "/repos/b")

	// Posting to project A with a CWD under project B's prefix should fail.
	body := strings.NewReader(`{"id":"sess-wt-3","cwd":"/repos/b/sub/dir"}`)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/projects/"+projAID+"/ai/sessions",
		body,
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d. body=%s",
			rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
}

// mustRun executes a shell command in dir, creating dir if necessary.
// It sets git identity env vars so git commands work in CI without global config.
func mustRun(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}
