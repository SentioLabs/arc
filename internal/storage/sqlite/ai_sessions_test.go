package sqlite_test

import (
	"context"
	"strings"
	"testing"

	"github.com/sentiolabs/arc/internal/types"
)

func TestAISessionStubMethods(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("CreateAISession returns not implemented", func(t *testing.T) {
		session := &types.AISession{ID: "test-session"}
		err := store.CreateAISession(ctx, session)
		if err == nil || !strings.Contains(err.Error(), "not implemented") {
			t.Errorf("CreateAISession() error = %v, want 'not implemented'", err)
		}
	})

	t.Run("GetAISession returns not implemented", func(t *testing.T) {
		_, err := store.GetAISession(ctx, "test-session")
		if err == nil || !strings.Contains(err.Error(), "not implemented") {
			t.Errorf("GetAISession() error = %v, want 'not implemented'", err)
		}
	})

	t.Run("ListAISessions returns not implemented", func(t *testing.T) {
		_, err := store.ListAISessions(ctx, 10, 0)
		if err == nil || !strings.Contains(err.Error(), "not implemented") {
			t.Errorf("ListAISessions() error = %v, want 'not implemented'", err)
		}
	})

	t.Run("DeleteAISession returns not implemented", func(t *testing.T) {
		err := store.DeleteAISession(ctx, "test-session")
		if err == nil || !strings.Contains(err.Error(), "not implemented") {
			t.Errorf("DeleteAISession() error = %v, want 'not implemented'", err)
		}
	})

	t.Run("CreateAIAgent returns not implemented", func(t *testing.T) {
		agent := &types.AIAgent{ID: "test-agent", SessionID: "test-session"}
		err := store.CreateAIAgent(ctx, agent)
		if err == nil || !strings.Contains(err.Error(), "not implemented") {
			t.Errorf("CreateAIAgent() error = %v, want 'not implemented'", err)
		}
	})

	t.Run("GetAIAgent returns not implemented", func(t *testing.T) {
		_, err := store.GetAIAgent(ctx, "test-agent")
		if err == nil || !strings.Contains(err.Error(), "not implemented") {
			t.Errorf("GetAIAgent() error = %v, want 'not implemented'", err)
		}
	})

	t.Run("ListAIAgents returns not implemented", func(t *testing.T) {
		_, err := store.ListAIAgents(ctx, "test-session")
		if err == nil || !strings.Contains(err.Error(), "not implemented") {
			t.Errorf("ListAIAgents() error = %v, want 'not implemented'", err)
		}
	})
}
