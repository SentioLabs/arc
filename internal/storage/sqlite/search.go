package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/sentiolabs/arc/internal/types"
)

// rebuildFTSForIssue rebuilds the FTS5 index entry for a single issue.
// It aggregates title, description, all comments/plans, and labels into one row.
// Errors are logged but not propagated — FTS sync is best-effort.
func (s *Store) rebuildFTSForIssue(ctx context.Context, issueID string) {
	issue, err := s.queries.GetIssue(ctx, issueID)
	if err != nil {
		log.Printf("fts: failed to get issue %s for reindex: %v", issueID, err)
		return
	}

	description := fromNullString(issue.Description)

	// Delete existing entry then insert new one.
	s.deleteFTSForIssue(ctx, issueID)

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO issues_fts(id, title, description)
		 VALUES (?, ?, ?)`,
		issueID, issue.Title, description,
	)
	if err != nil {
		log.Printf("fts: failed to insert issue %s into FTS index: %v", issueID, err)
	}
}

// deleteFTSForIssue removes an issue's entry from the FTS5 index.
func (s *Store) deleteFTSForIssue(ctx context.Context, issueID string) {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM issues_fts WHERE id = ?`, issueID,
	)
	if err != nil {
		log.Printf("fts: failed to delete issue %s from FTS index: %v", issueID, err)
	}
}

// populateFTS rebuilds the entire FTS5 index from scratch.
// Called on startup to reconcile the index against the issues table.
func (s *Store) populateFTS(ctx context.Context) {
	// Always recreate from scratch — this handles both clean state and corruption.
	s.recreateFTSTable(ctx)

	// Bulk-insert all issues into the fresh FTS table.
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO issues_fts(id, title, description)
		 SELECT id, title, COALESCE(description, '') FROM issues`)
	if err != nil {
		log.Printf("fts: failed to populate FTS index: %v", err)
		return
	}

	count, _ := result.RowsAffected()
	log.Printf("fts: populated index for %d issues", count)
}

// recreateFTSTable drops and recreates the FTS5 virtual table.
// Used to recover from a corrupted index (e.g. after table recreation in migrations).
// When an FTS5 table is corrupted, DROP TABLE can also fail, so we manually
// remove the shadow tables as a fallback.
func (s *Store) recreateFTSTable(ctx context.Context) {
	if _, err := s.db.ExecContext(ctx, `DROP TABLE IF EXISTS issues_fts`); err != nil {
		log.Printf("fts: DROP TABLE failed, removing shadow tables manually: %v", err)
		// FTS5 shadow tables follow the pattern <table>_<suffix>
		for _, suffix := range []string{"content", "docsize", "config", "data", "idx"} {
			s.db.ExecContext(ctx, `DROP TABLE IF EXISTS issues_fts_`+suffix) //nolint:errcheck // best-effort cleanup
		}
		// Try dropping the virtual table again now that shadow tables are gone
		s.db.ExecContext(ctx, `DROP TABLE IF EXISTS issues_fts`) //nolint:errcheck // may still fail
	}
	if _, err := s.db.ExecContext(ctx,
		`CREATE VIRTUAL TABLE IF NOT EXISTS issues_fts USING fts5(id, title, description, content=issues, content_rowid=rowid)`); err != nil {
		log.Printf("fts: failed to recreate FTS table: %v", err)
	}
}

// searchIssuesFTS searches for issues using FTS5 full-text search with BM25 ranking.
// Falls back to LIKE search if FTS5 MATCH fails (e.g. invalid query syntax).
func (s *Store) searchIssuesFTS(
	ctx context.Context, projectID, query string, limit, offset int,
) ([]*types.Issue, error) {
	ftsQuery := PrepareSearchQuery(query)

	rows, err := s.db.QueryContext(ctx, `
		SELECT i.id, i.project_id, i.title, i.description, i.status, i.priority,
		       i.issue_type, i.external_ref, i.rank,
		       i.created_at, i.updated_at, i.closed_at, i.close_reason,
		       bm25(issues_fts, 0.0, 10.0, 5.0) as relevance
		FROM issues_fts
		JOIN issues i ON i.id = issues_fts.id
		WHERE i.project_id = ?
		  AND issues_fts MATCH ?
		ORDER BY relevance
		LIMIT ? OFFSET ?
	`, projectID, ftsQuery, limit, offset)
	if err != nil {
		// Fall back to LIKE search on FTS failure
		log.Printf("fts: MATCH query failed (query=%q, fts=%q), falling back to LIKE: %v", query, ftsQuery, err)
		return s.searchIssuesLIKE(ctx, projectID, query, limit, offset)
	}
	defer rows.Close()

	var issues []*types.Issue
	for rows.Next() {
		issue, err := scanIssueRow(rows, true)
		if err != nil {
			return nil, fmt.Errorf("scan FTS result: %w", err)
		}
		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate FTS results: %w", err)
	}

	return issues, nil
}

// searchIssuesLIKE is the fallback search using LIKE pattern matching.
func (s *Store) searchIssuesLIKE(
	ctx context.Context, projectID, query string, limit, offset int,
) ([]*types.Issue, error) {
	searchPattern := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, title, description, status, priority,
		       issue_type, external_ref, rank,
		       created_at, updated_at, closed_at, close_reason
		FROM issues
		WHERE project_id = ?
		  AND (title LIKE ? OR description LIKE ?)
		ORDER BY updated_at DESC
		LIMIT ? OFFSET ?
	`, projectID, searchPattern, searchPattern, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("LIKE search: %w", err)
	}
	defer rows.Close()

	var issues []*types.Issue
	for rows.Next() {
		issue, err := scanIssueRow(rows, false)
		if err != nil {
			return nil, fmt.Errorf("scan LIKE result: %w", err)
		}
		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate LIKE results: %w", err)
	}

	return issues, nil
}

// scanIssueRow scans a row from a search query into a types.Issue.
// If hasRelevance is true, it also scans a trailing relevance float64 column.
func scanIssueRow(rows *sql.Rows, hasRelevance bool) (*types.Issue, error) {
	var (
		issue       types.Issue
		description sql.NullString
		externalRef sql.NullString
		closedAt    sql.NullTime
		closeReason sql.NullString
	)

	dest := []any{
		&issue.ID, &issue.ProjectID, &issue.Title, &description,
		&issue.Status, &issue.Priority, &issue.IssueType,
		&externalRef, &issue.Rank,
		&issue.CreatedAt, &issue.UpdatedAt, &closedAt, &closeReason,
	}

	if hasRelevance {
		var relevance sql.NullFloat64
		dest = append(dest, &relevance)
	}

	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}

	issue.Description = fromNullString(description)
	issue.ExternalRef = fromNullString(externalRef)
	issue.CloseReason = fromNullString(closeReason)
	issue.ClosedAt = fromNullTime(closedAt)

	return &issue, nil
}

// PrepareSearchQuery transforms user input into an FTS5 MATCH query.
// - Each unquoted term gets a `*` suffix for prefix matching
// - Quoted phrases are passed through for exact matching
// - Empty input returns empty string
func PrepareSearchQuery(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	var parts []string
	remaining := input

	for len(remaining) > 0 {
		remaining = strings.TrimSpace(remaining)
		if len(remaining) == 0 {
			break
		}

		var part string
		part, remaining = parseNextToken(remaining)
		if part != "" {
			parts = append(parts, part)
		}
	}

	return strings.Join(parts, " ")
}

// parseNextToken extracts the next search token from the input.
// Quoted phrases are returned as-is; unquoted terms get a `*` suffix.
// Returns the token and the remaining unparsed input.
func parseNextToken(s string) (token, rest string) {
	if s[0] == '"' {
		return parseQuotedPhrase(s)
	}

	end := strings.IndexAny(s, " \t\"")
	if end == -1 {
		return s + "*", ""
	}
	return s[:end] + "*", s[end:]
}

// parseQuotedPhrase extracts a quoted phrase from the input.
func parseQuotedPhrase(s string) (token, rest string) {
	end := strings.Index(s[1:], "\"")
	if end == -1 {
		// No closing quote — treat rest as a quoted phrase
		return s, ""
	}
	return s[:end+2], s[end+2:]
}
