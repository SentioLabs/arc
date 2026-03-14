package client_test

import (
	"testing"

	"github.com/sentiolabs/arc/internal/types"
)

func TestClientCreateAISession(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	session := &types.AISession{
		ID:             "test-session-123",
		TranscriptPath: "/tmp/transcript.jsonl",
		CWD:            "/home/user/project",
	}

	created, err := c.CreateAISession(session)
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

func TestClientGetAISession(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	// Create session first
	session := &types.AISession{
		ID:             "get-session-456",
		TranscriptPath: "/tmp/transcript.jsonl",
	}
	_, err := c.CreateAISession(session)
	if err != nil {
		t.Fatalf("CreateAISession failed: %v", err)
	}

	// Get session
	got, err := c.GetAISession("get-session-456")
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

	_, err := c.GetAISession("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestClientListAISessions(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	// Create multiple sessions
	for _, id := range []string{"list-a", "list-b", "list-c"} {
		_, err := c.CreateAISession(&types.AISession{ID: id})
		if err != nil {
			t.Fatalf("CreateAISession(%s) failed: %v", id, err)
		}
	}

	sessions, err := c.ListAISessions(10, 0)
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

	for _, id := range []string{"page-a", "page-b", "page-c"} {
		_, err := c.CreateAISession(&types.AISession{ID: id})
		if err != nil {
			t.Fatalf("CreateAISession(%s) failed: %v", id, err)
		}
	}

	sessions, err := c.ListAISessions(2, 0)
	if err != nil {
		t.Fatalf("ListAISessions failed: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("sessions count = %d, want 2", len(sessions))
	}
}

func TestClientDeleteAISession(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	_, err := c.CreateAISession(&types.AISession{ID: "delete-me"})
	if err != nil {
		t.Fatalf("CreateAISession failed: %v", err)
	}

	if err := c.DeleteAISession("delete-me"); err != nil {
		t.Fatalf("DeleteAISession failed: %v", err)
	}

	// Verify it's gone
	_, err = c.GetAISession("delete-me")
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestClientCreateAIAgent(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	// Create session first
	_, err := c.CreateAISession(&types.AISession{ID: "agent-session"})
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

	created, err := c.CreateAIAgent("agent-session", agent)
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

func TestClientListAIAgents(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	_, err := c.CreateAISession(&types.AISession{ID: "agents-list-session"})
	if err != nil {
		t.Fatalf("CreateAISession failed: %v", err)
	}

	for _, id := range []string{"ag-1", "ag-2"} {
		_, err := c.CreateAIAgent("agents-list-session", &types.AIAgent{
			ID:     id,
			Status: "completed",
		})
		if err != nil {
			t.Fatalf("CreateAIAgent(%s) failed: %v", id, err)
		}
	}

	agents, err := c.ListAIAgents("agents-list-session")
	if err != nil {
		t.Fatalf("ListAIAgents failed: %v", err)
	}

	if len(agents) != 2 {
		t.Errorf("agents count = %d, want 2", len(agents))
	}
}

func TestClientGetAIAgent(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	_, err := c.CreateAISession(&types.AISession{ID: "get-agent-session"})
	if err != nil {
		t.Fatalf("CreateAISession failed: %v", err)
	}

	_, err = c.CreateAIAgent("get-agent-session", &types.AIAgent{
		ID:          "get-agent-001",
		Description: "Agent to retrieve",
		Status:      "completed",
	})
	if err != nil {
		t.Fatalf("CreateAIAgent failed: %v", err)
	}

	agent, err := c.GetAIAgent("get-agent-session", "get-agent-001")
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

	_, err := c.CreateAISession(&types.AISession{ID: "no-agent-session"})
	if err != nil {
		t.Fatalf("CreateAISession failed: %v", err)
	}

	_, err = c.GetAIAgent("no-agent-session", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestClientGetAgentTranscript(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	_, err := c.CreateAISession(&types.AISession{
		ID:             "transcript-session",
		TranscriptPath: "/nonexistent/path.jsonl",
	})
	if err != nil {
		t.Fatalf("CreateAISession failed: %v", err)
	}

	// The transcript file doesn't exist, so we expect an error
	_, err = c.GetAgentTranscript("transcript-session", "agent-001")
	if err == nil {
		t.Fatal("expected error when transcript file does not exist")
	}
}

func TestClientGetAISessionIncludesAgents(t *testing.T) {
	c, cleanup := testClientServer(t)
	defer cleanup()

	_, err := c.CreateAISession(&types.AISession{ID: "session-with-agents"})
	if err != nil {
		t.Fatalf("CreateAISession failed: %v", err)
	}

	_, err = c.CreateAIAgent("session-with-agents", &types.AIAgent{
		ID:     "included-agent",
		Status: "running",
	})
	if err != nil {
		t.Fatalf("CreateAIAgent failed: %v", err)
	}

	session, err := c.GetAISession("session-with-agents")
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
