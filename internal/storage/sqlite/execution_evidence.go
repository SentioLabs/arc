package sqlite

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/storage/sqlite/db"
	"github.com/sentiolabs/arc/internal/types"
)

// checkExecutionExpected compares the context actually captured by the worker.
// Missing expectations remain compatible only for legacy unlinked work.
func (m *issueMutationTx) checkExecutionExpected(
	ctx context.Context,
	id string,
	expected *types.ExpectedGovernance,
) (*types.ExpectedGovernance, error) {
	issue, err := m.GetIssue(ctx, id)
	if err != nil {
		return nil, err
	}
	g, err := m.resolveGoverningPlan(ctx, issue.ProjectID, id)
	if err != nil {
		return nil, err
	}
	actual := &types.ExpectedGovernance{Governing: g, ContractVersion: issue.ContractVersion}
	if expected == nil {
		if g != nil {
			return nil, storage.ErrExecutionPrecondition
		}
		return actual, nil
	}
	if !sameExpected(*expected, *actual) {
		return nil, fmt.Errorf(
			"%w: %s current contract version %d",
			storage.ErrExecutionConflict,
			id,
			issue.ContractVersion,
		)
	}
	return actual, nil
}

// sameGovernance compares every source identity and ordered context reference.
// Empty nil and empty JSON arrays carry the same zero-ancestor meaning.
func sameGovernance(a, b *types.GoverningPlan) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.ContainerID == b.ContainerID && a.ContainerType == b.ContainerType &&
		a.Reference == b.Reference &&
		len(a.Context) == len(b.Context) &&
		(len(a.Context) == 0 || reflect.DeepEqual(a.Context, b.Context))
}

// sameExpected joins immutable execution provenance with scope-bearing version.
func sameExpected(a, b types.ExpectedGovernance) bool {
	return a.ContractVersion == b.ContractVersion && sameGovernance(a.Governing, b.Governing)
}

// UpdateIssueWithExpected captures both precondition validation and the post-write
// issue detail in the same transaction as status/claim changes.
func (s *Store) UpdateIssueWithExpected(
	ctx context.Context,
	id string,
	updates map[string]any,
	actor string,
	expected *types.ExpectedGovernance,
) (*types.IssueDetails, error) {
	var result *types.IssueDetails
	err := s.withIssueMutation(ctx, func(m *issueMutationTx) error {
		m.expected = expected
		if expected != nil && updates["status"] != string(types.StatusClosed) {
			if _, err := m.checkExecutionExpected(ctx, id, expected); err != nil {
				return err
			}
		}
		if err := m.UpdateIssue(ctx, id, updates, actor); err != nil {
			return err
		}
		issue, err := m.GetIssue(ctx, id)
		if err != nil {
			return err
		}
		g, err := m.resolveGoverningPlan(ctx, issue.ProjectID, id)
		if err != nil {
			return err
		}
		result = &types.IssueDetails{Issue: *issue, ResolvedGovernance: g}
		return nil
	})
	return result, err
}

// CloseIssueWithExpected passes captured context into the shared close transaction.
// Cascades cannot supply separate worker contexts for governed descendants.
//
//nolint:revive // Existing close parameters plus an explicit captured expectation.
func (s *Store) CloseIssueWithExpected(
	ctx context.Context,
	id, reason string,
	cascade bool,
	actor string,
	expected *types.ExpectedGovernance,
) error {
	return s.withIssueMutation(ctx, func(m *issueMutationTx) error {
		m.expected = expected
		return m.CloseIssue(ctx, id, reason, cascade, actor)
	})
}

// RecordExecutionEvidence validates before appending phase/session provenance.
// The immutable record retains the original project and full governance chain.
// Evidence never increments a task contract version or closes the issue.
// Missing expectations are allowed only for valid legacy unlinked work.
func (s *Store) RecordExecutionEvidence(
	ctx context.Context,
	projectID, id string,
	req types.ExecutionEvidenceRequest,
) (*storage.ExecutionEvidence, error) {
	if (req.Phase != "build" && req.Phase != "review" && req.Phase != "verify") ||
		strings.TrimSpace(req.Evidence) == "" {
		return nil, storage.ErrPlanInvalid
	}
	var result *storage.ExecutionEvidence
	err := s.withIssueMutation(ctx, func(m *issueMutationTx) error {
		i, err := m.GetIssue(ctx, id)
		if err != nil {
			return err
		}
		if i.ProjectID != projectID {
			return storage.ErrIssueNotFound
		}
		actual, err := m.checkExecutionExpected(ctx, id, req.Expected)
		if err != nil {
			return err
		}
		provenance := storage.PlanProvenanceFromContext(ctx)
		result = &storage.ExecutionEvidence{
			ID:        rand.Text(),
			ProjectID: projectID,
			IssueID:   id,
			Expected:  *actual,
			Phase:     req.Phase,
			Evidence:  req.Evidence,
			Actor:     provenance.Actor,
			SessionID: provenance.SessionID,
			CreatedAt: time.Now().UTC(),
		}
		body, err := json.Marshal(result)
		if err != nil {
			return err
		}
		_, err = m.tx.ExecContext(
			ctx,
			`INSERT INTO execution_evidence(id,project_id,issue_id,result,created_at) VALUES(?,?,?,?,?)`,
			result.ID,
			projectID,
			id,
			string(body),
			result.CreatedAt,
		)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListExecutionEvidence reads immutable capture records using current ownership.
// A plan-free merge can rehome lookup ownership without rewriting old provenance.
func (s *Store) ListExecutionEvidence(
	ctx context.Context,
	p, id string,
	limit, offset int,
) ([]storage.ExecutionEvidence, error) {
	return readGovernanceHistory[storage.ExecutionEvidence](
		ctx,
		s,
		p,
		id,
		func(m *issueMutationTx) ([]string, error) {
			if limit < 1 || limit > 200 || offset < 0 {
				return nil, storage.ErrPlanInvalid
			}
			return m.queries.ListExecutionEvidenceRecords(
				ctx,
				db.ListExecutionEvidenceRecordsParams{
					ProjectID: p,
					IssueID:   id,
					Limit:     int64(limit),
					Offset:    int64(offset),
				},
			)
		},
	)
}

// readGovernanceHistory verifies current project access and decodes retained
// results within one snapshot. Both history surfaces use generated bounded SQL;
// neither returns partial records if a retained payload cannot be decoded.
func readGovernanceHistory[T any](
	ctx context.Context,
	s *Store,
	p, id string,
	read func(*issueMutationTx) ([]string, error),
) ([]T, error) {
	result := []T{}
	err := s.withIssueMutation(ctx, func(m *issueMutationTx) error {
		issue, err := m.GetIssue(ctx, id)
		if err != nil {
			return err
		}
		if issue.ProjectID != p {
			return storage.ErrIssueNotFound
		}
		bodies, err := read(m)
		if err != nil {
			return err
		}
		for _, body := range bodies {
			var record T
			if err := json.Unmarshal([]byte(body), &record); err != nil {
				return err
			}
			result = append(result, record)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
