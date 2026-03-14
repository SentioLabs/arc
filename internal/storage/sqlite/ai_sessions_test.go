package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/sentiolabs/arc/internal/types"
)

func TestCreateAISession(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	session := &types.AISession{
		ID:             "session-001",
		TranscriptPath: "/tmp/transcripts/session-001.jsonl",
		CWD:            "/home/user/project",
		StartedAt:      time.Now().Truncate(time.Millisecond),
	}

	err := store.CreateAISession(ctx, session)
	if err != nil {
		t.Fatalf("CreateAISession() error = %v", err)
	}

	// Verify we can retrieve it and fields match
	got, err := store.GetAISession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetAISession() error = %v", err)
	}

	if got.ID != session.ID {
		t.Errorf("ID = %q, want %q", got.ID, session.ID)
	}
	if got.TranscriptPath != session.TranscriptPath {
		t.Errorf("TranscriptPath = %q, want %q", got.TranscriptPath, session.TranscriptPath)
	}
	if got.CWD != session.CWD {
		t.Errorf("CWD = %q, want %q", got.CWD, session.CWD)
	}
	if !got.StartedAt.Equal(session.StartedAt) {
		t.Errorf("StartedAt = %v, want %v", got.StartedAt, session.StartedAt)
	}
}

func TestGetAISession(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	session := &types.AISession{
		ID:             "session-get",
		TranscriptPath: "/tmp/transcripts/session-get.jsonl",
		CWD:            "/home/user/project",
		StartedAt:      time.Now().Truncate(time.Millisecond),
	}

	if err := store.CreateAISession(ctx, session); err != nil {
		t.Fatalf("CreateAISession() error = %v", err)
	}

	got, err := store.GetAISession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetAISession() error = %v", err)
	}

	if got.ID != session.ID {
		t.Errorf("ID = %q, want %q", got.ID, session.ID)
	}
	if got.TranscriptPath != session.TranscriptPath {
		t.Errorf("TranscriptPath = %q, want %q", got.TranscriptPath, session.TranscriptPath)
	}
}

func TestGetAISession_NotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	_, err := store.GetAISession(ctx, "nonexistent-session")
	if err == nil {
		t.Fatal("GetAISession() should return error for missing ID")
	}
}

func TestListAISessions(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create sessions with different started_at times
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	sessions := []*types.AISession{
		{ID: "session-list-1", TranscriptPath: "/t/1.jsonl", StartedAt: baseTime},
		{ID: "session-list-2", TranscriptPath: "/t/2.jsonl", StartedAt: baseTime.Add(1 * time.Hour)},
		{ID: "session-list-3", TranscriptPath: "/t/3.jsonl", StartedAt: baseTime.Add(2 * time.Hour)},
	}

	for _, s := range sessions {
		if err := store.CreateAISession(ctx, s); err != nil {
			t.Fatalf("CreateAISession() error = %v", err)
		}
	}

	// List all — should be newest first
	listed, err := store.ListAISessions(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListAISessions() error = %v", err)
	}

	if len(listed) != 3 {
		t.Fatalf("ListAISessions() returned %d sessions, want 3", len(listed))
	}

	// Verify newest first order
	if listed[0].ID != "session-list-3" {
		t.Errorf("first session ID = %q, want %q", listed[0].ID, "session-list-3")
	}
	if listed[2].ID != "session-list-1" {
		t.Errorf("last session ID = %q, want %q", listed[2].ID, "session-list-1")
	}

	// Test pagination: limit 2, offset 0
	page1, err := store.ListAISessions(ctx, 2, 0)
	if err != nil {
		t.Fatalf("ListAISessions(limit=2, offset=0) error = %v", err)
	}
	if len(page1) != 2 {
		t.Errorf("page1 length = %d, want 2", len(page1))
	}

	// Test pagination: limit 2, offset 2
	page2, err := store.ListAISessions(ctx, 2, 2)
	if err != nil {
		t.Fatalf("ListAISessions(limit=2, offset=2) error = %v", err)
	}
	if len(page2) != 1 {
		t.Errorf("page2 length = %d, want 1", len(page2))
	}
}

func TestDeleteAISession(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	session := &types.AISession{
		ID:             "session-delete",
		TranscriptPath: "/t/delete.jsonl",
		StartedAt:      time.Now().Truncate(time.Millisecond),
	}

	if err := store.CreateAISession(ctx, session); err != nil {
		t.Fatalf("CreateAISession() error = %v", err)
	}

	// Verify it exists
	_, err := store.GetAISession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetAISession() before delete error = %v", err)
	}

	// Delete
	if err := store.DeleteAISession(ctx, session.ID); err != nil {
		t.Fatalf("DeleteAISession() error = %v", err)
	}

	// Verify it's gone
	_, err = store.GetAISession(ctx, session.ID)
	if err == nil {
		t.Fatal("GetAISession() should return error after deletion")
	}
}

func TestDeleteAISession_CascadesAgents(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	session := &types.AISession{
		ID:             "session-cascade",
		TranscriptPath: "/t/cascade.jsonl",
		StartedAt:      time.Now().Truncate(time.Millisecond),
	}

	if err := store.CreateAISession(ctx, session); err != nil {
		t.Fatalf("CreateAISession() error = %v", err)
	}

	// Create agents linked to session
	agent := &types.AIAgent{
		ID:        "agent-cascade-1",
		SessionID: session.ID,
		Status:    "completed",
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	if err := store.CreateAIAgent(ctx, agent); err != nil {
		t.Fatalf("CreateAIAgent() error = %v", err)
	}

	// Verify agent exists
	_, err := store.GetAIAgent(ctx, agent.ID)
	if err != nil {
		t.Fatalf("GetAIAgent() before cascade delete error = %v", err)
	}

	// Delete session — should cascade to agents
	if err := store.DeleteAISession(ctx, session.ID); err != nil {
		t.Fatalf("DeleteAISession() error = %v", err)
	}

	// Verify agent is gone too
	_, err = store.GetAIAgent(ctx, agent.ID)
	if err == nil {
		t.Fatal("GetAIAgent() should return error after session cascade delete")
	}
}

func TestCreateAIAgent(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	session := &types.AISession{
		ID:             "session-agent",
		TranscriptPath: "/t/agent.jsonl",
		StartedAt:      time.Now().Truncate(time.Millisecond),
	}
	if err := store.CreateAISession(ctx, session); err != nil {
		t.Fatalf("CreateAISession() error = %v", err)
	}

	durationMs := 5000
	totalTokens := 1500
	toolUseCount := 3

	agent := &types.AIAgent{
		ID:           "agent-001",
		SessionID:    session.ID,
		Description:  "Test agent",
		Prompt:       "Do something useful",
		AgentType:    "implementer",
		Model:        "claude-sonnet-4-20250514",
		Status:       "completed",
		DurationMs:   &durationMs,
		TotalTokens:  &totalTokens,
		ToolUseCount: &toolUseCount,
		CreatedAt:    time.Now().Truncate(time.Millisecond),
	}

	err := store.CreateAIAgent(ctx, agent)
	if err != nil {
		t.Fatalf("CreateAIAgent() error = %v", err)
	}

	// Verify fields
	got, err := store.GetAIAgent(ctx, agent.ID)
	if err != nil {
		t.Fatalf("GetAIAgent() error = %v", err)
	}

	if got.ID != agent.ID {
		t.Errorf("ID = %q, want %q", got.ID, agent.ID)
	}
	if got.SessionID != agent.SessionID {
		t.Errorf("SessionID = %q, want %q", got.SessionID, agent.SessionID)
	}
	if got.Description != agent.Description {
		t.Errorf("Description = %q, want %q", got.Description, agent.Description)
	}
	if got.Prompt != agent.Prompt {
		t.Errorf("Prompt = %q, want %q", got.Prompt, agent.Prompt)
	}
	if got.AgentType != agent.AgentType {
		t.Errorf("AgentType = %q, want %q", got.AgentType, agent.AgentType)
	}
	if got.Model != agent.Model {
		t.Errorf("Model = %q, want %q", got.Model, agent.Model)
	}
	if got.Status != agent.Status {
		t.Errorf("Status = %q, want %q", got.Status, agent.Status)
	}
	if got.DurationMs == nil || *got.DurationMs != durationMs {
		t.Errorf("DurationMs = %v, want %d", got.DurationMs, durationMs)
	}
	if got.TotalTokens == nil || *got.TotalTokens != totalTokens {
		t.Errorf("TotalTokens = %v, want %d", got.TotalTokens, totalTokens)
	}
	if got.ToolUseCount == nil || *got.ToolUseCount != toolUseCount {
		t.Errorf("ToolUseCount = %v, want %d", got.ToolUseCount, toolUseCount)
	}
}

func TestGetAIAgent(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	session := &types.AISession{
		ID:             "session-getagent",
		TranscriptPath: "/t/getagent.jsonl",
		StartedAt:      time.Now().Truncate(time.Millisecond),
	}
	if err := store.CreateAISession(ctx, session); err != nil {
		t.Fatalf("CreateAISession() error = %v", err)
	}

	agent := &types.AIAgent{
		ID:        "agent-get",
		SessionID: session.ID,
		Status:    "running",
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	if err := store.CreateAIAgent(ctx, agent); err != nil {
		t.Fatalf("CreateAIAgent() error = %v", err)
	}

	got, err := store.GetAIAgent(ctx, agent.ID)
	if err != nil {
		t.Fatalf("GetAIAgent() error = %v", err)
	}

	if got.ID != agent.ID {
		t.Errorf("ID = %q, want %q", got.ID, agent.ID)
	}
	if got.SessionID != agent.SessionID {
		t.Errorf("SessionID = %q, want %q", got.SessionID, agent.SessionID)
	}
	if got.Status != agent.Status {
		t.Errorf("Status = %q, want %q", got.Status, agent.Status)
	}
}

func TestListAIAgents(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	session := &types.AISession{
		ID:             "session-listagents",
		TranscriptPath: "/t/listagents.jsonl",
		StartedAt:      time.Now().Truncate(time.Millisecond),
	}
	if err := store.CreateAISession(ctx, session); err != nil {
		t.Fatalf("CreateAISession() error = %v", err)
	}

	// Create agents with different created_at times
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	agents := []*types.AIAgent{
		{ID: "agent-list-1", SessionID: session.ID, Status: "completed", CreatedAt: baseTime},
		{ID: "agent-list-2", SessionID: session.ID, Status: "completed", CreatedAt: baseTime.Add(1 * time.Hour)},
		{ID: "agent-list-3", SessionID: session.ID, Status: "running", CreatedAt: baseTime.Add(2 * time.Hour)},
	}

	for _, a := range agents {
		if err := store.CreateAIAgent(ctx, a); err != nil {
			t.Fatalf("CreateAIAgent() error = %v", err)
		}
	}

	listed, err := store.ListAIAgents(ctx, session.ID)
	if err != nil {
		t.Fatalf("ListAIAgents() error = %v", err)
	}

	if len(listed) != 3 {
		t.Fatalf("ListAIAgents() returned %d agents, want 3", len(listed))
	}

	// Verify oldest first order
	if listed[0].ID != "agent-list-1" {
		t.Errorf("first agent ID = %q, want %q", listed[0].ID, "agent-list-1")
	}
	if listed[2].ID != "agent-list-3" {
		t.Errorf("last agent ID = %q, want %q", listed[2].ID, "agent-list-3")
	}
}
