package sqlite

import (
	"context"
	"fmt"

	"github.com/sentiolabs/arc/internal/types"
)

// CreateAISession creates a new AI session record.
func (s *Store) CreateAISession(_ context.Context, _ *types.AISession) error {
	return fmt.Errorf("not implemented")
}

// GetAISession retrieves an AI session by ID.
func (s *Store) GetAISession(_ context.Context, _ string) (*types.AISession, error) {
	return nil, fmt.Errorf("not implemented")
}

// ListAISessions returns AI sessions ordered by started_at descending.
func (s *Store) ListAISessions(_ context.Context, _, _ int) ([]*types.AISession, error) {
	return nil, fmt.Errorf("not implemented")
}

// DeleteAISession deletes an AI session by ID.
func (s *Store) DeleteAISession(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}

// CreateAIAgent creates a new AI agent record.
func (s *Store) CreateAIAgent(_ context.Context, _ *types.AIAgent) error {
	return fmt.Errorf("not implemented")
}

// GetAIAgent retrieves an AI agent by ID.
func (s *Store) GetAIAgent(_ context.Context, _ string) (*types.AIAgent, error) {
	return nil, fmt.Errorf("not implemented")
}

// ListAIAgents returns AI agents for a session ordered by created_at ascending.
func (s *Store) ListAIAgents(_ context.Context, _ string) ([]*types.AIAgent, error) {
	return nil, fmt.Errorf("not implemented")
}
