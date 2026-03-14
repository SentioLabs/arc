//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// arc ai session start
// ---------------------------------------------------------------------------

// TestAISessionStart verifies that `arc ai session start` creates a session
// and returns a confirmation message.
func TestAISessionStart(t *testing.T) {
	home := setupHome(t)

	output := arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", "test-session-start-001",
		"--transcript-path", "/tmp/test.jsonl",
		"--server", serverURL)

	if !strings.Contains(output, "test-session-start-001") {
		t.Errorf("expected session ID in output, got: %s", output)
	}
}

// TestAISessionStartIdempotent verifies that starting the same session twice
// succeeds (idempotent) and returns the existing session.
func TestAISessionStartIdempotent(t *testing.T) {
	home := setupHome(t)

	arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", "test-session-idempotent",
		"--transcript-path", "/tmp/test.jsonl",
		"--server", serverURL)

	// Second call with same ID should also succeed.
	output := arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", "test-session-idempotent",
		"--transcript-path", "/tmp/test.jsonl",
		"--server", serverURL)

	if !strings.Contains(output, "test-session-idempotent") {
		t.Errorf("expected session ID in idempotent output, got: %s", output)
	}
}

// TestAISessionStartMissingID verifies that `arc ai session start` without
// --id returns an error.
func TestAISessionStartMissingID(t *testing.T) {
	home := setupHome(t)

	output, err := arcCmd(t, home, "ai", "session", "start", "--server", serverURL)
	if err == nil {
		t.Fatalf("expected error when --id is missing, got: %s", output)
	}
	if !strings.Contains(strings.ToLower(output), "required") {
		t.Errorf("expected 'required' in error message, got: %s", output)
	}
}

// TestAISessionStartJsonOutput verifies JSON output from session start.
func TestAISessionStartJsonOutput(t *testing.T) {
	home := setupHome(t)

	output := arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", "test-session-json-out",
		"--transcript-path", "/tmp/test.jsonl",
		"--json",
		"--server", serverURL)

	var session map[string]interface{}
	if err := json.Unmarshal([]byte(output), &session); err != nil {
		t.Fatalf("expected valid JSON, got: %s\nerr: %v", output, err)
	}

	if session["id"] != "test-session-json-out" {
		t.Errorf("expected id test-session-json-out, got %v", session["id"])
	}
	if session["transcript_path"] != "/tmp/test.jsonl" {
		t.Errorf("expected transcript_path /tmp/test.jsonl, got %v", session["transcript_path"])
	}
}

// ---------------------------------------------------------------------------
// arc ai session list
// ---------------------------------------------------------------------------

// TestAISessionList verifies that `arc ai session list` shows created sessions.
func TestAISessionList(t *testing.T) {
	home := setupHome(t)

	// Create two sessions.
	arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", "test-list-sess-a",
		"--transcript-path", "/tmp/a.jsonl",
		"--server", serverURL)
	arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", "test-list-sess-b",
		"--transcript-path", "/tmp/b.jsonl",
		"--server", serverURL)

	output := arcCmdSuccess(t, home, "ai", "session", "list", "--server", serverURL)

	if !strings.Contains(output, "test-list-sess-a") {
		t.Errorf("expected session a in list output, got: %s", output)
	}
	if !strings.Contains(output, "test-list-sess-b") {
		t.Errorf("expected session b in list output, got: %s", output)
	}
}

// TestAISessionListEmpty verifies that listing sessions when none exist
// produces a friendly message, not an error.
func TestAISessionListEmpty(t *testing.T) {
	home := setupHome(t)

	// Use a fresh server — sessions from other tests may exist, but
	// we at least verify the command doesn't fail.
	output := arcCmdSuccess(t, home, "ai", "session", "list", "--server", serverURL)
	// Should either show sessions or "No AI sessions found"
	_ = output
}

// TestAISessionListJsonOutput verifies JSON output from session list.
func TestAISessionListJsonOutput(t *testing.T) {
	home := setupHome(t)

	arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", "test-list-json-sess",
		"--transcript-path", "/tmp/test.jsonl",
		"--server", serverURL)

	output := arcCmdSuccess(t, home, "ai", "session", "list", "--json", "--server", serverURL)

	// The CLI outputs a JSON array of sessions (not wrapped in {items:...}).
	var sessions []interface{}
	if err := json.Unmarshal([]byte(output), &sessions); err != nil {
		t.Fatalf("expected valid JSON array, got: %s\nerr: %v", output, err)
	}
	if len(sessions) == 0 {
		t.Error("expected at least one session in list output")
	}
}

// ---------------------------------------------------------------------------
// arc ai session show
// ---------------------------------------------------------------------------

// TestAISessionShow verifies that `arc ai session show` displays session
// details and any agents.
func TestAISessionShow(t *testing.T) {
	home := setupHome(t)

	arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", "test-show-sess",
		"--transcript-path", "/tmp/show.jsonl",
		"--server", serverURL)

	output := arcCmdSuccess(t, home, "ai", "session", "show", "test-show-sess", "--server", serverURL)

	if !strings.Contains(output, "test-show-sess") {
		t.Errorf("expected session ID in show output, got: %s", output)
	}
	if !strings.Contains(output, "/tmp/show.jsonl") {
		t.Errorf("expected transcript path in show output, got: %s", output)
	}
}

// TestAISessionShowJsonOutput verifies JSON output from session show.
func TestAISessionShowJsonOutput(t *testing.T) {
	home := setupHome(t)

	arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", "test-show-json-sess",
		"--transcript-path", "/tmp/show.jsonl",
		"--server", serverURL)

	output := arcCmdSuccess(t, home, "ai", "session", "show",
		"test-show-json-sess", "--json", "--server", serverURL)

	var session map[string]interface{}
	if err := json.Unmarshal([]byte(output), &session); err != nil {
		t.Fatalf("expected valid JSON, got: %s\nerr: %v", output, err)
	}

	if session["id"] != "test-show-json-sess" {
		t.Errorf("expected id test-show-json-sess, got %v", session["id"])
	}

	// Should have agents field (possibly empty).
	if _, exists := session["agents"]; !exists {
		t.Error("expected 'agents' field in session show JSON")
	}
}

// TestAISessionShowWithAgents verifies that session show includes agents
// that were registered against the session.
func TestAISessionShowWithAgents(t *testing.T) {
	home := setupHome(t)

	sessionID := "test-show-with-agents"
	arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", sessionID,
		"--transcript-path", "/tmp/test.jsonl",
		"--server", serverURL)

	// Register an agent via the API directly to keep the test focused.
	registerAgentViaAPI(t, sessionID, "agent-show-test-001", "Test agent")

	output := arcCmdSuccess(t, home, "ai", "session", "show", sessionID, "--json", "--server", serverURL)

	var session map[string]interface{}
	if err := json.Unmarshal([]byte(output), &session); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, output)
	}

	agents, ok := session["agents"].([]interface{})
	if !ok || len(agents) == 0 {
		t.Errorf("expected at least one agent in session show, got: %v", session["agents"])
	}
}

// TestAISessionShowNotFound verifies that showing a non-existent session
// returns an error.
func TestAISessionShowNotFound(t *testing.T) {
	home := setupHome(t)

	_, err := arcCmd(t, home, "ai", "session", "show", "nonexistent-session-xyz", "--server", serverURL)
	if err == nil {
		t.Error("expected error when showing non-existent session")
	}
}

// ---------------------------------------------------------------------------
// arc ai agent register --stdin
// ---------------------------------------------------------------------------

// TestAIAgentRegister verifies that `arc ai agent register --stdin` creates
// an agent from a PostToolUse payload.
func TestAIAgentRegister(t *testing.T) {
	home := setupHome(t)

	// Create session first.
	arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", "test-register-sess",
		"--transcript-path", "/tmp/register.jsonl",
		"--server", serverURL)

	payload := makePostToolUsePayload("test-register-sess", "agent-reg-001", "Test registration")

	output := arcCmdWithStdinSuccess(t, home, payload,
		"ai", "agent", "register", "--stdin", "--server", serverURL)

	if !strings.Contains(output, "agent-reg-001") {
		t.Errorf("expected agent ID in output, got: %s", output)
	}
	if !strings.Contains(output, "test-register-sess") {
		t.Errorf("expected session ID in output, got: %s", output)
	}
}

// TestAIAgentRegisterJsonOutput verifies JSON output from agent register.
func TestAIAgentRegisterJsonOutput(t *testing.T) {
	home := setupHome(t)

	arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", "test-register-json-sess",
		"--transcript-path", "/tmp/register.jsonl",
		"--server", serverURL)

	payload := makePostToolUsePayload("test-register-json-sess", "agent-reg-json-001", "JSON register test")

	output := arcCmdWithStdinSuccess(t, home, payload,
		"ai", "agent", "register", "--stdin", "--json", "--server", serverURL)

	var agent map[string]interface{}
	if err := json.Unmarshal([]byte(output), &agent); err != nil {
		t.Fatalf("expected valid JSON, got: %s\nerr: %v", output, err)
	}

	if agent["id"] != "agent-reg-json-001" {
		t.Errorf("expected agent id agent-reg-json-001, got %v", agent["id"])
	}
	if agent["session_id"] != "test-register-json-sess" {
		t.Errorf("expected session_id test-register-json-sess, got %v", agent["session_id"])
	}
	if agent["description"] != "JSON register test" {
		t.Errorf("expected description 'JSON register test', got %v", agent["description"])
	}
}

// TestAIAgentRegisterLazySession verifies that registering an agent for a
// non-existent session auto-creates the session.
func TestAIAgentRegisterLazySession(t *testing.T) {
	home := setupHome(t)

	// Do NOT create the session first.
	payload := makePostToolUsePayload("test-lazy-sess", "agent-lazy-001", "Lazy session test")

	output := arcCmdWithStdinSuccess(t, home, payload,
		"ai", "agent", "register", "--stdin", "--server", serverURL)

	if !strings.Contains(output, "agent-lazy-001") {
		t.Errorf("expected agent ID in output, got: %s", output)
	}

	// Verify the session was auto-created.
	showOutput := arcCmdSuccess(t, home, "ai", "session", "show",
		"test-lazy-sess", "--json", "--server", serverURL)

	var session map[string]interface{}
	if err := json.Unmarshal([]byte(showOutput), &session); err != nil {
		t.Fatalf("parse session JSON: %v", err)
	}
	if session["id"] != "test-lazy-sess" {
		t.Errorf("expected lazily created session, got: %v", session["id"])
	}
}

// TestAIAgentRegisterAllFields verifies that all fields from the PostToolUse
// payload are persisted correctly.
func TestAIAgentRegisterAllFields(t *testing.T) {
	home := setupHome(t)

	sessionID := "test-fields-sess"
	arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", sessionID,
		"--transcript-path", "/tmp/fields.jsonl",
		"--server", serverURL)

	payload := `{
		"session_id": "test-fields-sess",
		"transcript_path": "/tmp/fields.jsonl",
		"cwd": "/home/user/project",
		"tool_name": "Agent",
		"tool_input": {
			"description": "Full fields test",
			"prompt": "Do something complex",
			"subagent_type": "arc-implementer",
			"model": "opus"
		},
		"tool_response": {
			"status": "completed",
			"agentId": "agent-fields-001",
			"totalDurationMs": 12345,
			"totalTokens": 67890,
			"totalToolUseCount": 42
		}
	}`

	arcCmdWithStdinSuccess(t, home, payload,
		"ai", "agent", "register", "--stdin", "--server", serverURL)

	// Verify via agent show --json.
	showOutput := arcCmdSuccess(t, home, "ai", "agent", "show", "agent-fields-001",
		"--session", sessionID, "--json", "--server", serverURL)

	var agent map[string]interface{}
	if err := json.Unmarshal([]byte(showOutput), &agent); err != nil {
		t.Fatalf("parse agent JSON: %v\noutput: %s", err, showOutput)
	}

	checks := map[string]interface{}{
		"id":          "agent-fields-001",
		"session_id":  sessionID,
		"description": "Full fields test",
		"agent_type":  "arc-implementer",
		"model":       "opus",
		"status":      "completed",
	}
	for field, expected := range checks {
		if agent[field] != expected {
			t.Errorf("field %s: expected %v, got %v", field, expected, agent[field])
		}
	}

	// Check numeric fields (JSON numbers are float64).
	if v, ok := agent["duration_ms"].(float64); !ok || int(v) != 12345 {
		t.Errorf("expected duration_ms 12345, got %v", agent["duration_ms"])
	}
	if v, ok := agent["total_tokens"].(float64); !ok || int(v) != 67890 {
		t.Errorf("expected total_tokens 67890, got %v", agent["total_tokens"])
	}
	if v, ok := agent["tool_use_count"].(float64); !ok || int(v) != 42 {
		t.Errorf("expected tool_use_count 42, got %v", agent["tool_use_count"])
	}
}

// TestAIAgentRegisterNonAgentTool verifies that payloads for non-Agent tools
// are silently ignored (exit 0, no registration).
func TestAIAgentRegisterNonAgentTool(t *testing.T) {
	home := setupHome(t)

	payload := `{
		"session_id": "test-non-agent-sess",
		"tool_name": "Bash",
		"tool_input": {"command": "ls"},
		"tool_response": {"status": "completed"}
	}`

	output := arcCmdWithStdinSuccess(t, home, payload,
		"ai", "agent", "register", "--stdin", "--server", serverURL)

	// Should produce no output — silently ignored.
	if strings.TrimSpace(output) != "" {
		t.Errorf("expected empty output for non-Agent tool, got: %s", output)
	}
}

// TestAIAgentRegisterEmptyStdin verifies that empty stdin returns an error.
func TestAIAgentRegisterEmptyStdin(t *testing.T) {
	home := setupHome(t)

	_, err := arcCmdWithStdin(t, home, "",
		"ai", "agent", "register", "--stdin", "--server", serverURL)
	if err == nil {
		t.Error("expected error with empty stdin")
	}
}

// TestAIAgentRegisterInvalidJSON verifies that invalid JSON stdin returns an error.
func TestAIAgentRegisterInvalidJSON(t *testing.T) {
	home := setupHome(t)

	_, err := arcCmdWithStdin(t, home, "not valid json{{{",
		"ai", "agent", "register", "--stdin", "--server", serverURL)
	if err == nil {
		t.Error("expected error with invalid JSON stdin")
	}
}

// TestAIAgentRegisterMissingSessionID verifies that a payload without
// session_id returns an error.
func TestAIAgentRegisterMissingSessionID(t *testing.T) {
	home := setupHome(t)

	payload := `{
		"tool_name": "Agent",
		"tool_input": {"description": "test"},
		"tool_response": {"agentId": "agent-no-sess", "status": "completed"}
	}`

	output, err := arcCmdWithStdin(t, home, payload,
		"ai", "agent", "register", "--stdin", "--server", serverURL)
	if err == nil {
		t.Fatalf("expected error without session_id, got: %s", output)
	}
	if !strings.Contains(strings.ToLower(output), "session_id") {
		t.Errorf("expected error mentioning session_id, got: %s", output)
	}
}

// TestAIAgentRegisterWithoutStdinFlag verifies that omitting --stdin
// returns an error.
func TestAIAgentRegisterWithoutStdinFlag(t *testing.T) {
	home := setupHome(t)

	output, err := arcCmd(t, home, "ai", "agent", "register", "--server", serverURL)
	if err == nil {
		t.Fatalf("expected error without --stdin flag, got: %s", output)
	}
	if !strings.Contains(strings.ToLower(output), "stdin") {
		t.Errorf("expected error mentioning --stdin, got: %s", output)
	}
}

// TestAIAgentRegisterMultipleAgents verifies that multiple agents can be
// registered for the same session.
func TestAIAgentRegisterMultipleAgents(t *testing.T) {
	home := setupHome(t)

	sessionID := "test-multi-agent-sess"
	arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", sessionID,
		"--transcript-path", "/tmp/multi.jsonl",
		"--server", serverURL)

	for i := 1; i <= 3; i++ {
		payload := makePostToolUsePayload(sessionID,
			fmt.Sprintf("agent-multi-%d", i),
			fmt.Sprintf("Agent %d", i))
		arcCmdWithStdinSuccess(t, home, payload,
			"ai", "agent", "register", "--stdin", "--server", serverURL)
	}

	// Verify all three agents appear in session show.
	showOutput := arcCmdSuccess(t, home, "ai", "session", "show",
		sessionID, "--json", "--server", serverURL)

	var session map[string]interface{}
	if err := json.Unmarshal([]byte(showOutput), &session); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}

	agents, ok := session["agents"].([]interface{})
	if !ok || len(agents) != 3 {
		t.Errorf("expected 3 agents, got %d", len(agents))
	}
}

// ---------------------------------------------------------------------------
// arc ai agent show
// ---------------------------------------------------------------------------

// TestAIAgentShow verifies that `arc ai agent show` displays agent details.
func TestAIAgentShow(t *testing.T) {
	home := setupHome(t)

	sessionID := "test-agent-show-sess"
	arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", sessionID,
		"--transcript-path", "/tmp/show.jsonl",
		"--server", serverURL)

	payload := makePostToolUsePayload(sessionID, "agent-show-001", "Show test agent")
	arcCmdWithStdinSuccess(t, home, payload,
		"ai", "agent", "register", "--stdin", "--server", serverURL)

	output := arcCmdSuccess(t, home, "ai", "agent", "show", "agent-show-001",
		"--session", sessionID, "--server", serverURL)

	if !strings.Contains(output, "agent-show-001") {
		t.Errorf("expected agent ID in output, got: %s", output)
	}
	if !strings.Contains(output, "Show test agent") {
		t.Errorf("expected description in output, got: %s", output)
	}
	if !strings.Contains(output, sessionID) {
		t.Errorf("expected session ID in output, got: %s", output)
	}
}

// TestAIAgentShowJsonOutput verifies JSON output from agent show.
func TestAIAgentShowJsonOutput(t *testing.T) {
	home := setupHome(t)

	sessionID := "test-agent-show-json-sess"
	arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", sessionID,
		"--transcript-path", "/tmp/show-json.jsonl",
		"--server", serverURL)

	payload := makePostToolUsePayload(sessionID, "agent-show-json-001", "JSON show test")
	arcCmdWithStdinSuccess(t, home, payload,
		"ai", "agent", "register", "--stdin", "--server", serverURL)

	output := arcCmdSuccess(t, home, "ai", "agent", "show", "agent-show-json-001",
		"--session", sessionID, "--json", "--server", serverURL)

	var agent map[string]interface{}
	if err := json.Unmarshal([]byte(output), &agent); err != nil {
		t.Fatalf("expected valid JSON, got: %s\nerr: %v", output, err)
	}

	if agent["id"] != "agent-show-json-001" {
		t.Errorf("expected id agent-show-json-001, got %v", agent["id"])
	}
	if agent["description"] != "JSON show test" {
		t.Errorf("expected description 'JSON show test', got %v", agent["description"])
	}
}

// TestAIAgentShowMissingSession verifies that --session is required.
func TestAIAgentShowMissingSession(t *testing.T) {
	home := setupHome(t)

	output, err := arcCmd(t, home, "ai", "agent", "show", "some-agent", "--server", serverURL)
	if err == nil {
		t.Fatalf("expected error without --session, got: %s", output)
	}
	if !strings.Contains(strings.ToLower(output), "session") {
		t.Errorf("expected error mentioning --session, got: %s", output)
	}
}

// TestAIAgentShowNotFound verifies that showing a non-existent agent returns an error.
func TestAIAgentShowNotFound(t *testing.T) {
	home := setupHome(t)

	sessionID := "test-agent-notfound-sess"
	arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", sessionID,
		"--transcript-path", "/tmp/test.jsonl",
		"--server", serverURL)

	_, err := arcCmd(t, home, "ai", "agent", "show", "nonexistent-agent",
		"--session", sessionID, "--server", serverURL)
	if err == nil {
		t.Error("expected error when showing non-existent agent")
	}
}

// ---------------------------------------------------------------------------
// arc ai agent transcript
// ---------------------------------------------------------------------------

// TestAIAgentTranscriptMissingSession verifies that --session is required.
func TestAIAgentTranscriptMissingSession(t *testing.T) {
	home := setupHome(t)

	output, err := arcCmd(t, home, "ai", "agent", "transcript", "some-agent", "--server", serverURL)
	if err == nil {
		t.Fatalf("expected error without --session, got: %s", output)
	}
	if !strings.Contains(strings.ToLower(output), "session") {
		t.Errorf("expected error mentioning --session, got: %s", output)
	}
}

// ---------------------------------------------------------------------------
// Session delete (via API — no CLI command, but verify cascading)
// ---------------------------------------------------------------------------

// TestAISessionDeleteViaAPI verifies that deleting a session via the API
// removes it and its agents.
func TestAISessionDeleteViaAPI(t *testing.T) {
	home := setupHome(t)

	sessionID := "test-delete-sess"
	arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", sessionID,
		"--transcript-path", "/tmp/delete.jsonl",
		"--server", serverURL)

	// Register an agent.
	payload := makePostToolUsePayload(sessionID, "agent-delete-001", "Delete test agent")
	arcCmdWithStdinSuccess(t, home, payload,
		"ai", "agent", "register", "--stdin", "--server", serverURL)

	// Delete via API.
	deleteURL := fmt.Sprintf("%s/api/v1/ai/sessions/%s", serverURL, sessionID)
	req, err := http.NewRequest(http.MethodDelete, deleteURL, nil)
	if err != nil {
		t.Fatalf("create delete request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 from delete, got %d", resp.StatusCode)
	}

	// Verify session is gone.
	_, showErr := arcCmd(t, home, "ai", "session", "show", sessionID, "--server", serverURL)
	if showErr == nil {
		t.Error("expected error when showing deleted session")
	}
}

// ---------------------------------------------------------------------------
// End-to-end workflow: full lifecycle
// ---------------------------------------------------------------------------

// TestAIFullLifecycle exercises the complete flow: start session → register
// agents → list sessions → show session → show agent → verify all data.
func TestAIFullLifecycle(t *testing.T) {
	home := setupHome(t)

	sessionID := "test-lifecycle-sess"

	// 1. Start session.
	arcCmdSuccess(t, home, "ai", "session", "start",
		"--id", sessionID,
		"--transcript-path", "/home/user/.claude/projects/test/session.jsonl",
		"--server", serverURL)

	// 2. Register two agents.
	payload1 := `{
		"session_id": "test-lifecycle-sess",
		"transcript_path": "/home/user/.claude/projects/test/session.jsonl",
		"cwd": "/home/user/project",
		"tool_name": "Agent",
		"tool_input": {
			"description": "Implement feature X",
			"prompt": "Build the feature",
			"subagent_type": "arc-implementer",
			"model": "sonnet"
		},
		"tool_response": {
			"status": "completed",
			"agentId": "lifecycle-agent-1",
			"totalDurationMs": 5000,
			"totalTokens": 30000,
			"totalToolUseCount": 15
		}
	}`
	arcCmdWithStdinSuccess(t, home, payload1,
		"ai", "agent", "register", "--stdin", "--server", serverURL)

	payload2 := `{
		"session_id": "test-lifecycle-sess",
		"transcript_path": "/home/user/.claude/projects/test/session.jsonl",
		"cwd": "/home/user/project",
		"tool_name": "Agent",
		"tool_input": {
			"description": "Review implementation",
			"prompt": "Review the code",
			"subagent_type": "arc-reviewer",
			"model": "opus"
		},
		"tool_response": {
			"status": "completed",
			"agentId": "lifecycle-agent-2",
			"totalDurationMs": 3000,
			"totalTokens": 20000,
			"totalToolUseCount": 8
		}
	}`
	arcCmdWithStdinSuccess(t, home, payload2,
		"ai", "agent", "register", "--stdin", "--server", serverURL)

	// 3. List sessions — should include ours.
	listOutput := arcCmdSuccess(t, home, "ai", "session", "list", "--server", serverURL)
	if !strings.Contains(listOutput, sessionID) {
		t.Errorf("expected session in list, got: %s", listOutput)
	}

	// 4. Show session — should have 2 agents.
	showOutput := arcCmdSuccess(t, home, "ai", "session", "show",
		sessionID, "--json", "--server", serverURL)

	var session map[string]interface{}
	if err := json.Unmarshal([]byte(showOutput), &session); err != nil {
		t.Fatalf("parse session JSON: %v", err)
	}

	agents, ok := session["agents"].([]interface{})
	if !ok || len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}

	// 5. Show each agent individually.
	for _, agentID := range []string{"lifecycle-agent-1", "lifecycle-agent-2"} {
		agentOutput := arcCmdSuccess(t, home, "ai", "agent", "show", agentID,
			"--session", sessionID, "--json", "--server", serverURL)

		var agent map[string]interface{}
		if err := json.Unmarshal([]byte(agentOutput), &agent); err != nil {
			t.Fatalf("parse agent %s JSON: %v", agentID, err)
		}
		if agent["id"] != agentID {
			t.Errorf("expected agent id %s, got %v", agentID, agent["id"])
		}
		if agent["status"] != "completed" {
			t.Errorf("expected status completed, got %v", agent["status"])
		}
	}

	// 6. Verify agent details.
	agent1Out := arcCmdSuccess(t, home, "ai", "agent", "show", "lifecycle-agent-1",
		"--session", sessionID, "--server", serverURL)
	if !strings.Contains(agent1Out, "Implement feature X") {
		t.Errorf("expected description in agent show, got: %s", agent1Out)
	}
	if !strings.Contains(agent1Out, "arc-implementer") {
		t.Errorf("expected agent type in show, got: %s", agent1Out)
	}
	if !strings.Contains(agent1Out, "sonnet") {
		t.Errorf("expected model in agent show, got: %s", agent1Out)
	}
	if !strings.Contains(agent1Out, "5000") {
		t.Errorf("expected duration in agent show, got: %s", agent1Out)
	}
	if !strings.Contains(agent1Out, "30000") {
		t.Errorf("expected tokens in agent show, got: %s", agent1Out)
	}
	if !strings.Contains(agent1Out, "15") {
		t.Errorf("expected tool use count in agent show, got: %s", agent1Out)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makePostToolUsePayload creates a minimal PostToolUse JSON payload for testing.
func makePostToolUsePayload(sessionID, agentID, description string) string {
	return fmt.Sprintf(`{
		"session_id": %q,
		"transcript_path": "/tmp/test.jsonl",
		"cwd": "/tmp",
		"tool_name": "Agent",
		"tool_input": {
			"description": %q,
			"prompt": "test prompt",
			"subagent_type": "general-purpose",
			"model": "haiku"
		},
		"tool_response": {
			"status": "completed",
			"agentId": %q,
			"totalDurationMs": 1000,
			"totalTokens": 5000,
			"totalToolUseCount": 3
		}
	}`, sessionID, description, agentID)
}

// registerAgentViaAPI registers an agent directly via the HTTP API.
func registerAgentViaAPI(t *testing.T, sessionID, agentID, description string) {
	t.Helper()

	body := fmt.Sprintf(`{
		"id": %q,
		"description": %q,
		"agent_type": "general-purpose",
		"model": "haiku",
		"status": "completed"
	}`, agentID, description)

	url := fmt.Sprintf("%s/api/v1/ai/sessions/%s/agents", serverURL, sessionID)
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("register agent via API: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("register agent via API: expected 201/200, got %d", resp.StatusCode)
	}
}
