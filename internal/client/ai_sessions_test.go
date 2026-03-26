package client_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/types"
)

func TestClientCreateAISessionProjectScopedPath(t *testing.T) {
	// Verify that CreateAISession sends requests to /api/v1/projects/{projectId}/ai/sessions
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"sess-1","project_id":"proj-1","started_at":"2025-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	c := client.New(ts.URL)
	_, err := c.CreateAISession("proj-1", &types.AISession{
		ID: "sess-1",
	})
	if err != nil {
		t.Fatalf("CreateAISession failed: %v", err)
	}

	want := "/api/v1/projects/proj-1/ai/sessions"
	if gotPath != want {
		t.Errorf("request path = %q, want %q", gotPath, want)
	}
}

func TestClientGetAISessionProjectScopedPath(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"sess-1","project_id":"proj-1","started_at":"2025-01-01T00:00:00Z","agents":[]}`))
	}))
	defer ts.Close()

	c := client.New(ts.URL)
	_, err := c.GetAISession("proj-1", "sess-1")
	if err != nil {
		t.Fatalf("GetAISession failed: %v", err)
	}

	want := "/api/v1/projects/proj-1/ai/sessions/sess-1"
	if gotPath != want {
		t.Errorf("request path = %q, want %q", gotPath, want)
	}
}

func TestClientListAISessionsProjectScopedPath(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	c := client.New(ts.URL)
	_, err := c.ListAISessions("proj-1", 10, 0)
	if err != nil {
		t.Fatalf("ListAISessions failed: %v", err)
	}

	want := "/api/v1/projects/proj-1/ai/sessions"
	if gotPath != want {
		t.Errorf("request path = %q, want %q", gotPath, want)
	}
}

func TestClientDeleteAISessionProjectScopedPath(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := client.New(ts.URL)
	err := c.DeleteAISession("proj-1", "sess-1")
	if err != nil {
		t.Fatalf("DeleteAISession failed: %v", err)
	}

	want := "/api/v1/projects/proj-1/ai/sessions/sess-1"
	if gotPath != want {
		t.Errorf("request path = %q, want %q", gotPath, want)
	}
}

func TestClientCreateAIAgentProjectScopedPath(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ag-1","session_id":"sess-1","status":"running"}`))
	}))
	defer ts.Close()

	c := client.New(ts.URL)
	_, err := c.CreateAIAgent("proj-1", "sess-1", &types.AIAgent{
		ID:     "ag-1",
		Status: "running",
	})
	if err != nil {
		t.Fatalf("CreateAIAgent failed: %v", err)
	}

	want := "/api/v1/projects/proj-1/ai/sessions/sess-1/agents"
	if gotPath != want {
		t.Errorf("request path = %q, want %q", gotPath, want)
	}
}

func TestClientListAIAgentsProjectScopedPath(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	c := client.New(ts.URL)
	_, err := c.ListAIAgents("proj-1", "sess-1")
	if err != nil {
		t.Fatalf("ListAIAgents failed: %v", err)
	}

	want := "/api/v1/projects/proj-1/ai/sessions/sess-1/agents"
	if gotPath != want {
		t.Errorf("request path = %q, want %q", gotPath, want)
	}
}

func TestClientGetAIAgentProjectScopedPath(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ag-1","session_id":"sess-1","status":"running"}`))
	}))
	defer ts.Close()

	c := client.New(ts.URL)
	_, err := c.GetAIAgent("proj-1", "sess-1", "ag-1")
	if err != nil {
		t.Fatalf("GetAIAgent failed: %v", err)
	}

	want := "/api/v1/projects/proj-1/ai/sessions/sess-1/agents/ag-1"
	if gotPath != want {
		t.Errorf("request path = %q, want %q", gotPath, want)
	}
}

func TestClientGetAgentTranscriptProjectScopedPath(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	c := client.New(ts.URL)
	_, err := c.GetAgentTranscript("proj-1", "sess-1", "ag-1")
	if err != nil {
		t.Fatalf("GetAgentTranscript failed: %v", err)
	}

	want := "/api/v1/projects/proj-1/ai/sessions/sess-1/agents/ag-1/transcript"
	if gotPath != want {
		t.Errorf("request path = %q, want %q", gotPath, want)
	}
}

// Integration tests using real server
func TestClientCreateAISessionIntegration(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	// Register a workspace so the CWD resolves
	cwd := "/home/user/project"
	_, err := c.CreateWorkspace(proj.ID, client.CreateWorkspaceRequest{
		Path:  cwd,
		Label: "test-workspace",
	})
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	session := &types.AISession{
		ID:             "test-session-123",
		TranscriptPath: "/tmp/transcript.jsonl",
		CWD:            cwd,
	}

	created, err := c.CreateAISession(proj.ID, session)
	if err != nil {
		t.Fatalf("CreateAISession failed: %v", err)
	}

	if created.ID != session.ID {
		t.Errorf("ID = %q, want %q", created.ID, session.ID)
	}
	if created.TranscriptPath != session.TranscriptPath {
		t.Errorf("TranscriptPath = %q, want %q", created.TranscriptPath, session.TranscriptPath)
	}
	if created.CWD != session.CWD {
		t.Errorf("CWD = %q, want %q", created.CWD, session.CWD)
	}
	if created.StartedAt.IsZero() {
		t.Error("StartedAt should not be zero")
	}
}

func TestClientGetAISessionIntegration(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	session := &types.AISession{
		ID:             "get-session-456",
		TranscriptPath: "/tmp/transcript.jsonl",
	}
	_, err := c.CreateAISession(proj.ID, session)
	if err != nil {
		t.Fatalf("CreateAISession failed: %v", err)
	}

	got, err := c.GetAISession(proj.ID, "get-session-456")
	if err != nil {
		t.Fatalf("GetAISession failed: %v", err)
	}

	if got.ID != session.ID {
		t.Errorf("ID = %q, want %q", got.ID, session.ID)
	}
	if got.Agents == nil {
		t.Error("Agents field should be non-nil (empty slice)")
	}
}

func TestClientGetAISessionNotFound(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	_, err := c.GetAISession(proj.ID, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestClientListAISessionsIntegration(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	for _, id := range []string{"list-a", "list-b", "list-c"} {
		_, err := c.CreateAISession(proj.ID, &types.AISession{ID: id})
		if err != nil {
			t.Fatalf("CreateAISession(%s) failed: %v", id, err)
		}
	}

	sessions, err := c.ListAISessions(proj.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListAISessions failed: %v", err)
	}

	if len(sessions) != 3 {
		t.Errorf("sessions count = %d, want 3", len(sessions))
	}
}

func TestClientListAISessionsPagination(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	for _, id := range []string{"page-a", "page-b", "page-c"} {
		_, err := c.CreateAISession(proj.ID, &types.AISession{ID: id})
		if err != nil {
			t.Fatalf("CreateAISession(%s) failed: %v", id, err)
		}
	}

	sessions, err := c.ListAISessions(proj.ID, 2, 0)
	if err != nil {
		t.Fatalf("ListAISessions failed: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("sessions count = %d, want 2", len(sessions))
	}
}

func TestClientDeleteAISessionIntegration(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	_, err := c.CreateAISession(proj.ID, &types.AISession{ID: "delete-me"})
	if err != nil {
		t.Fatalf("CreateAISession failed: %v", err)
	}

	if err := c.DeleteAISession(proj.ID, "delete-me"); err != nil {
		t.Fatalf("DeleteAISession failed: %v", err)
	}

	_, err = c.GetAISession(proj.ID, "delete-me")
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestClientCreateAIAgentIntegration(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	_, err := c.CreateAISession(proj.ID, &types.AISession{ID: "agent-session"})
	if err != nil {
		t.Fatalf("CreateAISession failed: %v", err)
	}

	agent := &types.AIAgent{
		ID:          "agent-001",
		Description: "Test agent",
		AgentType:   "implementer",
		Model:       "claude-4",
		Status:      "running",
	}

	created, err := c.CreateAIAgent(proj.ID, "agent-session", agent)
	if err != nil {
		t.Fatalf("CreateAIAgent failed: %v", err)
	}

	if created.ID != agent.ID {
		t.Errorf("ID = %q, want %q", created.ID, agent.ID)
	}
	if created.SessionID != "agent-session" {
		t.Errorf("SessionID = %q, want %q", created.SessionID, "agent-session")
	}
	if created.Description != agent.Description {
		t.Errorf("Description = %q, want %q", created.Description, agent.Description)
	}
	if created.AgentType != agent.AgentType {
		t.Errorf("AgentType = %q, want %q", created.AgentType, agent.AgentType)
	}
}

func TestClientListAIAgentsIntegration(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	_, err := c.CreateAISession(proj.ID, &types.AISession{ID: "agents-list-session"})
	if err != nil {
		t.Fatalf("CreateAISession failed: %v", err)
	}

	for _, id := range []string{"ag-1", "ag-2"} {
		_, err := c.CreateAIAgent(proj.ID, "agents-list-session", &types.AIAgent{
			ID:     id,
			Status: "completed",
		})
		if err != nil {
			t.Fatalf("CreateAIAgent(%s) failed: %v", id, err)
		}
	}

	agents, err := c.ListAIAgents(proj.ID, "agents-list-session")
	if err != nil {
		t.Fatalf("ListAIAgents failed: %v", err)
	}

	if len(agents) != 2 {
		t.Errorf("agents count = %d, want 2", len(agents))
	}
}

func TestClientGetAIAgentIntegration(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	_, err := c.CreateAISession(proj.ID, &types.AISession{ID: "get-agent-session"})
	if err != nil {
		t.Fatalf("CreateAISession failed: %v", err)
	}

	_, err = c.CreateAIAgent(proj.ID, "get-agent-session", &types.AIAgent{
		ID:          "get-agent-001",
		Description: "Agent to retrieve",
		Status:      "completed",
	})
	if err != nil {
		t.Fatalf("CreateAIAgent failed: %v", err)
	}

	agent, err := c.GetAIAgent(proj.ID, "get-agent-session", "get-agent-001")
	if err != nil {
		t.Fatalf("GetAIAgent failed: %v", err)
	}

	if agent.ID != "get-agent-001" {
		t.Errorf("ID = %q, want %q", agent.ID, "get-agent-001")
	}
	if agent.Description != "Agent to retrieve" {
		t.Errorf("Description = %q, want %q", agent.Description, "Agent to retrieve")
	}
}

func TestClientGetAIAgentNotFound(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	_, err := c.CreateAISession(proj.ID, &types.AISession{ID: "no-agent-session"})
	if err != nil {
		t.Fatalf("CreateAISession failed: %v", err)
	}

	_, err = c.GetAIAgent(proj.ID, "no-agent-session", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestClientGetAgentTranscriptIntegration(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	_, err := c.CreateAISession(proj.ID, &types.AISession{
		ID:             "transcript-session",
		TranscriptPath: "/nonexistent/path.jsonl",
	})
	if err != nil {
		t.Fatalf("CreateAISession failed: %v", err)
	}

	// The transcript file doesn't exist, so we expect an error
	_, err = c.GetAgentTranscript(proj.ID, "transcript-session", "agent-001")
	if err == nil {
		t.Fatal("expected error when transcript file does not exist")
	}
}

func TestClientGetAISessionIncludesAgents(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	proj := createTestProjectClient(t, c)

	_, err := c.CreateAISession(proj.ID, &types.AISession{ID: "session-with-agents"})
	if err != nil {
		t.Fatalf("CreateAISession failed: %v", err)
	}

	_, err = c.CreateAIAgent(proj.ID, "session-with-agents", &types.AIAgent{
		ID:     "included-agent",
		Status: "running",
	})
	if err != nil {
		t.Fatalf("CreateAIAgent failed: %v", err)
	}

	session, err := c.GetAISession(proj.ID, "session-with-agents")
	if err != nil {
		t.Fatalf("GetAISession failed: %v", err)
	}

	if len(session.Agents) != 1 {
		t.Fatalf("Agents count = %d, want 1", len(session.Agents))
	}
	if session.Agents[0].ID != "included-agent" {
		t.Errorf("Agent ID = %q, want %q", session.Agents[0].ID, "included-agent")
	}
}
