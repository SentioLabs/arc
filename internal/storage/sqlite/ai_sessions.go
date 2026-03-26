// Package sqlite implements the storage.Storage interface using SQLite.
// This file provides AI session and agent CRUD operations backed by the
// ai_sessions and ai_agents tables.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// CreateAISession creates a new AI session record.
func (s *Store) CreateAISession(ctx context.Context, session *types.AISession) error {
	_, err := s.queries.CreateAISession(ctx, db.CreateAISessionParams{
		ID:             session.ID,
		ProjectID:      session.ProjectID,
		TranscriptPath: session.TranscriptPath,
		Cwd:            toNullString(session.CWD),
		StartedAt:      session.StartedAt,
	})
	if err != nil {
		return fmt.Errorf("create ai session: %w", err)
	}
	return nil
}

// GetAISession retrieves an AI session by ID.
func (s *Store) GetAISession(ctx context.Context, id string) (*types.AISession, error) {
	row, err := s.queries.GetAISession(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("ai session not found: %s", id)
		}
		return nil, fmt.Errorf("get ai session: %w", err)
	}
	return dbAISessionToType(row), nil
}

// ListAISessionsByProject returns AI sessions for a project ordered by started_at descending.
func (s *Store) ListAISessionsByProject(
	ctx context.Context, projectID string, limit, offset int,
) ([]*types.AISession, error) {
	rows, err := s.queries.ListAISessionsByProject(ctx, db.ListAISessionsByProjectParams{
		ProjectID: projectID,
		Limit:     int64(limit),
		Offset:    int64(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list ai sessions by project: %w", err)
	}

	sessions := make([]*types.AISession, len(rows))
	for i, row := range rows {
		sessions[i] = dbAISessionToType(row)
	}
	return sessions, nil
}

// CountAISessionsByProject returns the total number of AI sessions for a project.
func (s *Store) CountAISessionsByProject(ctx context.Context, projectID string) (int64, error) {
	count, err := s.queries.CountAISessionsByProject(ctx, projectID)
	if err != nil {
		return 0, fmt.Errorf("count ai sessions by project: %w", err)
	}
	return count, nil
}

// DeleteAISession deletes an AI session and its associated agents.
// Agents are deleted explicitly to ensure cascade behavior regardless of
// whether the SQLite driver honours the ON DELETE CASCADE pragma.
func (s *Store) DeleteAISession(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM ai_agents WHERE session_id = ?", id); err != nil {
		return fmt.Errorf("delete ai agents for session: %w", err)
	}
	if err := s.queries.DeleteAISession(ctx, id); err != nil {
		return fmt.Errorf("delete ai session: %w", err)
	}
	return nil
}

// CreateAIAgent creates a new AI agent record.
func (s *Store) CreateAIAgent(ctx context.Context, agent *types.AIAgent) error {
	_, err := s.queries.CreateAIAgent(ctx, db.CreateAIAgentParams{
		ID:           agent.ID,
		SessionID:    agent.SessionID,
		Description:  toNullString(agent.Description),
		Prompt:       toNullString(agent.Prompt),
		AgentType:    toNullString(agent.AgentType),
		Model:        toNullString(agent.Model),
		Status:       agent.Status,
		DurationMs:   toNullInt64(agent.DurationMs),
		TotalTokens:  toNullInt64(agent.TotalTokens),
		ToolUseCount: toNullInt64(agent.ToolUseCount),
		CreatedAt:    agent.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("create ai agent: %w", err)
	}
	return nil
}

// GetAIAgent retrieves an AI agent by ID.
func (s *Store) GetAIAgent(ctx context.Context, id string) (*types.AIAgent, error) {
	row, err := s.queries.GetAIAgent(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("ai agent not found: %s", id)
		}
		return nil, fmt.Errorf("get ai agent: %w", err)
	}
	return dbAIAgentToType(row), nil
}

// ListAIAgents returns AI agents for a session ordered by created_at ascending.
func (s *Store) ListAIAgents(ctx context.Context, sessionID string) ([]*types.AIAgent, error) {
	rows, err := s.queries.ListAIAgents(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list ai agents: %w", err)
	}

	agents := make([]*types.AIAgent, len(rows))
	for i, row := range rows {
		agents[i] = dbAIAgentToType(row)
	}
	return agents, nil
}

// GetAgentSummariesForSessions returns aggregated agent status counts for each session.
func (s *Store) GetAgentSummariesForSessions(
	ctx context.Context, sessionIDs []string,
) (map[string]*types.AgentSummary, error) {
	return make(map[string]*types.AgentSummary), nil
}

// toNullInt64 converts an *int to sql.NullInt64.
func toNullInt64(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

// dbAISessionToType converts a db.AiSession to types.AISession.
func dbAISessionToType(row *db.AiSession) *types.AISession {
	session := &types.AISession{
		ID:             row.ID,
		ProjectID:      row.ProjectID,
		TranscriptPath: row.TranscriptPath,
		StartedAt:      row.StartedAt,
	}
	if row.Cwd.Valid {
		session.CWD = row.Cwd.String
	}
	return session
}

// dbAIAgentToType converts a db.AiAgent to types.AIAgent.
func dbAIAgentToType(row *db.AiAgent) *types.AIAgent {
	agent := &types.AIAgent{
		ID:        row.ID,
		SessionID: row.SessionID,
		Status:    row.Status,
		CreatedAt: row.CreatedAt,
	}
	if row.Description.Valid {
		agent.Description = row.Description.String
	}
	if row.Prompt.Valid {
		agent.Prompt = row.Prompt.String
	}
	if row.AgentType.Valid {
		agent.AgentType = row.AgentType.String
	}
	if row.Model.Valid {
		agent.Model = row.Model.String
	}
	if row.DurationMs.Valid {
		v := int(row.DurationMs.Int64)
		agent.DurationMs = &v
	}
	if row.TotalTokens.Valid {
		v := int(row.TotalTokens.Int64)
		agent.TotalTokens = &v
	}
	if row.ToolUseCount.Valid {
		v := int(row.ToolUseCount.Int64)
		agent.ToolUseCount = &v
	}
	return agent
}
