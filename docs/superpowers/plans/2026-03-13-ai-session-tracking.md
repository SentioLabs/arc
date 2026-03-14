# AI Session Tracking Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `ai_session_id` field to issues so Claude Code sessions can be tracked, viewed, and resumed.

**Architecture:** New nullable `ai_session_id TEXT` column on the `issues` table. `arc update --take` flag reads session ID from `$ARC_SESSION_ID` env var (with `--session-id` override) and atomically sets the session ID + status. `arc prime` reads hook stdin JSON to persist the session ID via `CLAUDE_ENV_FILE`.

**Tech Stack:** Go, SQLite, sqlc, Echo, Cobra, OpenAPI codegen, Svelte/TypeScript (types only)

**Spec:** `docs/superpowers/specs/2026-03-13-ai-session-tracking-design.md`

---

## Chunk 1: Data Layer

### Task 1: Migration and Schema

**Files:**
- Create: `internal/storage/sqlite/migrations/011_ai_session_id.sql`
- Modify: `internal/storage/sqlite/db/schema.sql:34-49`

- [ ] **Step 1: Create migration file**

```sql
-- internal/storage/sqlite/migrations/011_ai_session_id.sql
ALTER TABLE issues ADD COLUMN ai_session_id TEXT;
```

- [ ] **Step 2: Update schema.sql reference**

Add `ai_session_id TEXT,` after the `assignee TEXT,` line (line 42) in `internal/storage/sqlite/db/schema.sql`:

```sql
    assignee TEXT,
    ai_session_id TEXT,
    external_ref TEXT,
```

- [ ] **Step 3: Commit**

```bash
git add internal/storage/sqlite/migrations/011_ai_session_id.sql internal/storage/sqlite/db/schema.sql
git commit -m "feat: add ai_session_id column to issues table"
```

### Task 2: sqlc Queries

**Files:**
- Modify: `internal/storage/sqlite/db/queries/issues.sql`

- [ ] **Step 1: Add `ai_session_id` to CreateIssue**

Update the `CreateIssue` query to include the new column. Change lines 1-6 from:

```sql
-- name: CreateIssue :exec
INSERT INTO issues (
    id, project_id, title, description,
    status, priority, issue_type, assignee, external_ref,
    created_at, updated_at, closed_at, close_reason
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
```

To:

```sql
-- name: CreateIssue :exec
INSERT INTO issues (
    id, project_id, title, description,
    status, priority, issue_type, assignee, ai_session_id, external_ref,
    created_at, updated_at, closed_at, close_reason
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
```

- [ ] **Step 2: Add `ai_session_id` to ListIssuesFiltered SELECT and WHERE**

Update the `ListIssuesFiltered` query. Change the SELECT to include `i.ai_session_id` and add a filter clause:

```sql
-- name: ListIssuesFiltered :many
SELECT i.id, i.project_id, i.title, i.description, i.status, i.priority,
       i.issue_type, i.assignee, i.ai_session_id, i.external_ref, i.rank,
       i.created_at, i.updated_at, i.closed_at, i.close_reason
FROM issues i
LEFT JOIN dependencies d ON d.issue_id = i.id AND d.type = 'parent-child'
WHERE i.project_id = sqlc.arg('project_id')
  AND (sqlc.narg('status') IS NULL OR i.status = sqlc.narg('status'))
  AND (sqlc.narg('issue_type') IS NULL OR i.issue_type = sqlc.narg('issue_type'))
  AND (sqlc.narg('assignee') IS NULL OR i.assignee = sqlc.narg('assignee'))
  AND (sqlc.narg('ai_session_id') IS NULL OR i.ai_session_id = sqlc.narg('ai_session_id'))
  AND (sqlc.narg('priority') IS NULL OR i.priority = sqlc.narg('priority'))
  AND (sqlc.narg('parent_id') IS NULL OR d.depends_on_id = sqlc.narg('parent_id'))
ORDER BY i.priority ASC, i.updated_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');
```

- [ ] **Step 3: Add UpdateIssueAISessionID query**

Add after the `UpdateIssueExternalRef` query:

```sql
-- name: UpdateIssueAISessionID :exec
UPDATE issues SET ai_session_id = ?, updated_at = ? WHERE id = ?;
```

- [ ] **Step 4: Run `make gen` and verify**

```bash
make gen
```

Expected: sqlc regenerates `internal/storage/sqlite/db/issues.sql.go` and `internal/storage/sqlite/db/models.go`. The `db.Issue` struct should now have `AiSessionID sql.NullString`. `ListIssuesFilteredParams` should have a new `AiSessionID` field.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/sqlite/db/queries/issues.sql internal/storage/sqlite/db/
git commit -m "feat: add ai_session_id to sqlc queries and regenerate"
```

### Task 3: Go Types

**Files:**
- Modify: `internal/types/types.go:50-82` (Issue struct)
- Modify: `internal/types/types.go:309-321` (IssueFilter struct)

- [ ] **Step 1: Add AISessionID to Issue struct**

Add after the `Assignee` field (line 67):

```go
	// Assignment
	Assignee string `json:"assignee,omitempty"`

	// AI Session Tracking
	AISessionID string `json:"ai_session_id,omitempty"` // Claude Code session UUID
```

- [ ] **Step 2: Add AISessionID to IssueFilter**

Add after the `Assignee` field (line 314):

```go
	Assignee    *string    // Filter by assignee
	AISessionID *string    // Filter by AI session ID
```

- [ ] **Step 3: Verify build**

```bash
go build ./...
```

Expected: Build succeeds (storage layer will fail until next task — that's expected if running standalone, but `make gen` in Task 2 should have already produced the matching types).

- [ ] **Step 4: Commit**

```bash
git add internal/types/types.go
git commit -m "feat: add AISessionID to Issue and IssueFilter types"
```

### Task 4: Storage Layer

**Files:**
- Modify: `internal/storage/sqlite/issues.go:111-125` (CreateIssue call site)
- Modify: `internal/storage/sqlite/issues.go:229-290` (UpdateIssue switch)
- Modify: `internal/storage/sqlite/issues.go:192-213` (ListIssues filter params)
- Modify: `internal/storage/sqlite/issues.go:462-479` (dbIssueToType)

- [ ] **Step 1: Update CreateIssue call site**

Add `AiSessionID` to the `CreateIssueParams` at line 119 (after `Assignee`):

```go
	err = s.queries.CreateIssue(ctx, db.CreateIssueParams{
		ID:          issue.ID,
		ProjectID:   issue.ProjectID,
		Title:       issue.Title,
		Description: toNullString(issue.Description),
		Status:      string(issue.Status),
		Priority:    int64(issue.Priority),
		IssueType:   string(issue.IssueType),
		Assignee:    toNullString(issue.Assignee),
		AiSessionID: toNullString(issue.AISessionID),
		ExternalRef: toNullString(issue.ExternalRef),
		CreatedAt:   now,
		UpdatedAt:   now,
		ClosedAt:    toNullTime(issue.ClosedAt),
		CloseReason: toNullString(issue.CloseReason),
	})
```

- [ ] **Step 2: Add `ai_session_id` case to UpdateIssue switch**

Add after the `"assignee"` case (after line 272):

```go
		case "ai_session_id":
			err = s.queries.UpdateIssueAISessionID(ctx, db.UpdateIssueAISessionIDParams{
				AiSessionID: toNullString(value.(string)),
				UpdatedAt:   now,
				ID:          id,
			})
```

- [ ] **Step 3: Add AISessionID to ListIssues filter params**

Add after the `Assignee` filter block (after line 207):

```go
	if filter.AISessionID != nil {
		params.AiSessionID = *filter.AISessionID
	}
```

- [ ] **Step 4: Update dbIssueToType**

Add `AISessionID` mapping (after `Assignee` line 472):

```go
func dbIssueToType(row *db.Issue) *types.Issue {
	return &types.Issue{
		ID:          row.ID,
		ProjectID:   row.ProjectID,
		Title:       row.Title,
		Description: fromNullString(row.Description),
		Status:      types.Status(row.Status),
		Priority:    int(row.Priority),
		Rank:        int(row.Rank),
		IssueType:   types.IssueType(row.IssueType),
		Assignee:    fromNullString(row.Assignee),
		AISessionID: fromNullString(row.AiSessionID),
		ExternalRef: fromNullString(row.ExternalRef),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		ClosedAt:    fromNullTime(row.ClosedAt),
		CloseReason: fromNullString(row.CloseReason),
	}
}
```

- [ ] **Step 5: Verify build**

```bash
go build ./...
```

Expected: Clean build.

- [ ] **Step 6: Run existing tests**

```bash
go test ./internal/storage/sqlite/... -v -count=1
```

Expected: All existing tests pass (new field is optional/nullable).

- [ ] **Step 7: Commit**

```bash
git add internal/storage/sqlite/issues.go
git commit -m "feat: wire ai_session_id through storage layer"
```

## Chunk 2: API and CLI Layer

### Task 5: OpenAPI Spec

**Files:**
- Modify: `api/openapi.yaml:1049-1110` (Issue schema)
- Modify: `api/openapi.yaml:1233-1256` (CreateIssueRequest)
- Modify: `api/openapi.yaml:1257-1276` (UpdateIssueRequest)
- Modify: `api/openapi.yaml` (list endpoint query params)

- [ ] **Step 1: Add `ai_session_id` to Issue schema**

Add after `assignee` (line 1084):

```yaml
        assignee:
          type: string
        ai_session_id:
          type: string
          description: AI coding session UUID (e.g., Claude Code session ID)
```

- [ ] **Step 2: Add `ai_session_id` to CreateIssueRequest**

Add after `assignee` (line 1253):

```yaml
        assignee:
          type: string
        ai_session_id:
          type: string
          description: AI coding session UUID
```

- [ ] **Step 3: Add `ai_session_id` to UpdateIssueRequest**

Add after `assignee` (line 1274):

```yaml
        assignee:
          type: string
        ai_session_id:
          type: string
          description: AI coding session UUID
```

- [ ] **Step 4: Add `ai_session_id` query parameter to list endpoints**

Find the list issues endpoint query parameters (search for `name: assignee` under the list path) and add after the assignee param:

```yaml
        - name: ai_session_id
          in: query
          description: Filter by AI session ID
          schema:
            type: string
```

Do the same for the ready/search endpoints if they have query params.

- [ ] **Step 5: Regenerate OpenAPI code**

```bash
make gen
```

Expected: `internal/api/openapi.gen.go` and `web/src/lib/api/types.ts` are regenerated.

- [ ] **Step 6: Commit**

```bash
git add api/openapi.yaml internal/api/openapi.gen.go web/src/lib/api/types.ts
git commit -m "feat: add ai_session_id to OpenAPI spec and regenerate"
```

### Task 6: API Handlers

**Files:**
- Modify: `internal/api/issues.go:18-39` (request types)
- Modify: `internal/api/issues.go` (listIssues handler — ai_session_id query param)
- Modify: `internal/api/issues.go` (createIssue handler — wire AISessionID)
- Modify: `internal/api/issues.go` (updateIssue handler — wire AISessionID)

- [ ] **Step 1: Add AISessionID to request types**

In `createIssueRequest` (after `Assignee` line 25):

```go
type createIssueRequest struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
	Priority    int    `json:"priority,omitempty"`
	IssueType   string `json:"issue_type,omitempty"`
	Assignee    string `json:"assignee,omitempty"`
	AISessionID string `json:"ai_session_id,omitempty"`
	ExternalRef string `json:"external_ref,omitempty"`
	ParentID    string `json:"parent_id,omitempty"`
}
```

In `updateIssueRequest` (after `Assignee` line 37):

```go
type updateIssueRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Status      *string `json:"status,omitempty"`
	Priority    *int    `json:"priority,omitempty"`
	IssueType   *string `json:"issue_type,omitempty"`
	Assignee    *string `json:"assignee,omitempty"`
	AISessionID *string `json:"ai_session_id,omitempty"`
	ExternalRef *string `json:"external_ref,omitempty"`
}
```

- [ ] **Step 2: Wire AISessionID in listIssues handler**

Find where `assignee` query param is read (search for `c.QueryParam("assignee")` in the listIssues function) and add after it:

```go
	if aiSessionID := c.QueryParam("ai_session_id"); aiSessionID != "" {
		filter.AISessionID = &aiSessionID
	}
```

- [ ] **Step 3: Wire AISessionID in createIssue handler**

Find where `Assignee: req.Assignee` is set in the create handler and add:

```go
		Assignee:    req.Assignee,
		AISessionID: req.AISessionID,
```

- [ ] **Step 4: Wire AISessionID in updateIssue handler**

Find where `req.Assignee` is handled in the update handler and add after it:

```go
	if req.AISessionID != nil {
		updates["ai_session_id"] = *req.AISessionID
	}
```

- [ ] **Step 5: Verify build and tests**

```bash
go build ./...
go test ./internal/api/... -v -count=1
```

Expected: Clean build, existing tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/api/issues.go
git commit -m "feat: wire ai_session_id through API handlers"
```

### Task 7: CLI Update Command — `--take` Flag

**Files:**
- Modify: `cmd/arc/main.go:828-902` (updateCmd)

- [ ] **Step 1: Add flags to updateCmd init**

Add after the `--stdin` flag (line 901):

```go
	updateCmd.Flags().Bool("take", false, "Take this issue for the current AI session (sets ai_session_id + status=in_progress)")
	updateCmd.Flags().String("session-id", "", "Explicit AI session ID (used with --take)")
```

- [ ] **Step 2: Add --take logic to updateCmd RunE**

Add after the description handling block (after line 873, before the `len(updates) == 0` check):

```go
		// Handle --take flag
		take, _ := cmd.Flags().GetBool("take")
		sessionID, _ := cmd.Flags().GetString("session-id")

		if sessionID != "" && !take {
			return errors.New("--session-id requires --take")
		}

		if take {
			// Resolve session ID: explicit flag > env var > error
			if sessionID == "" {
				sessionID = os.Getenv("ARC_SESSION_ID")
			}
			if sessionID == "" {
				return errors.New("no session ID available — set ARC_SESSION_ID or pass --session-id")
			}
			updates["ai_session_id"] = sessionID
			// Set status to in_progress unless user explicitly passed --status
			if !cmd.Flags().Changed("status") {
				updates["status"] = "in_progress"
			}
		}
```

- [ ] **Step 3: Verify build**

```bash
go build ./cmd/arc/...
```

- [ ] **Step 4: Manual smoke test**

```bash
# Should fail with no session ID
./bin/arc update <some-id> --take 2>&1 | grep "no session ID"

# Should fail with --session-id but no --take
./bin/arc update <some-id> --session-id abc 2>&1 | grep "requires --take"

# Should work with explicit session ID
ARC_SESSION_ID=test-123 ./bin/arc update <some-id> --take
```

- [ ] **Step 5: Commit**

```bash
git add cmd/arc/main.go
git commit -m "feat: add --take and --session-id flags to update command"
```

### Task 8: CLI Show Command — Display AI Session

**Files:**
- Modify: `cmd/arc/main.go:760-768` (show output)

- [ ] **Step 1: Add AISessionID to show output**

Find the `if details.Assignee != ""` block (line 766-768) and add after it:

```go
		if details.Assignee != "" {
			fmt.Printf("Assignee: %s\n", details.Assignee)
		}
		if details.AISessionID != "" {
			fmt.Printf("AI Session: %s\n", details.AISessionID)
		}
```

- [ ] **Step 2: Verify build**

```bash
go build ./cmd/arc/...
```

- [ ] **Step 3: Commit**

```bash
git add cmd/arc/main.go
git commit -m "feat: display ai_session_id in arc show output"
```

## Chunk 3: Hook Integration

### Task 9: `arc prime` Stdin Handling

**Files:**
- Modify: `cmd/arc/prime.go:1-16` (imports)
- Modify: `cmd/arc/prime.go:42-102` (primeCmd Run function)

- [ ] **Step 1: Add stdin parsing function**

Add a new function to `cmd/arc/prime.go` before the `init()` function:

```go
// hookInput represents the JSON payload sent by Claude Code to hooks via stdin.
type hookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	HookEventName  string `json:"hook_event_name"`
	Source         string `json:"source"`
}

// uuidPattern validates session IDs as UUIDs to prevent shell injection.
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// parseHookInput attempts to parse Claude Code hook JSON from the given reader.
// Returns empty string if not parseable or session_id is invalid.
// Separated from stdin detection for testability.
func parseHookInput(r io.Reader) string {
	// Read with a 2-second deadline to avoid hanging
	done := make(chan []byte, 1)
	go func() {
		data, err := io.ReadAll(r)
		if err != nil {
			done <- nil
			return
		}
		done <- data
	}()

	var data []byte
	select {
	case data = <-done:
	case <-time.After(2 * time.Second):
		return ""
	}

	if len(data) == 0 {
		return ""
	}

	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return ""
	}

	if !uuidPattern.MatchString(input.SessionID) {
		return ""
	}

	return input.SessionID
}

// readHookStdin attempts to read and parse Claude Code hook JSON from stdin.
// Returns empty string if stdin is a TTY, not parseable, or session_id is invalid.
func readHookStdin() string {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice != 0 {
		return "" // stdin is a TTY, not a pipe
	}
	return parseHookInput(os.Stdin)
}

// persistSessionID writes the session ID to CLAUDE_ENV_FILE for the session's
// Bash environment. Idempotent — replaces any existing ARC_SESSION_ID line.
func persistSessionID(sessionID string) {
	envFile := os.Getenv("CLAUDE_ENV_FILE")
	if envFile == "" || sessionID == "" {
		return
	}

	line := fmt.Sprintf("export ARC_SESSION_ID=%s\n", sessionID)

	// Read existing content to check for existing ARC_SESSION_ID
	existing, err := os.ReadFile(envFile)
	if err == nil {
		lines := strings.Split(string(existing), "\n")
		var filtered []string
		for _, l := range lines {
			if !strings.HasPrefix(l, "export ARC_SESSION_ID=") {
				filtered = append(filtered, l)
			}
		}
		content := strings.Join(filtered, "\n")
		if !strings.HasSuffix(content, "\n") && content != "" {
			content += "\n"
		}
		content += line
		_ = os.WriteFile(envFile, []byte(content), 0o600)
		return
	}

	// File doesn't exist yet, create it
	_ = os.WriteFile(envFile, []byte(line), 0o600)
}
```

- [ ] **Step 2: Update imports**

Add `"encoding/json"`, `"regexp"`, `"strings"`, and `"time"` to the imports in `cmd/arc/prime.go` (some may already be present).

- [ ] **Step 3: Call stdin handling at the start of primeCmd.Run**

At the very beginning of the `Run` function (line 43, before the cwd check), add:

```go
		// Read hook stdin and persist session ID if available
		sessionID := readHookStdin()
		if sessionID != "" {
			persistSessionID(sessionID)
		}

		// Also check env var (may have been set by a previous SessionStart hook)
		if sessionID == "" {
			sessionID = os.Getenv("ARC_SESSION_ID")
		}
```

- [ ] **Step 4: Add primeData struct and update output functions**

Add a template data struct:

```go
type primeData struct {
	SessionID string
	Role      string // only used by teammate template
}
```

Update function signatures and implementations:

```go
func outputPrimeContext(w io.Writer, mcpMode bool, sessionID string) error {
	if mcpMode {
		return outputMCPContext(w, sessionID)
	}
	return outputCLIContext(w, sessionID)
}

func outputMCPContext(w io.Writer, sessionID string) error {
	return tmplMCP.Execute(w, primeData{SessionID: sessionID})
}

func outputCLIContext(w io.Writer, sessionID string) error {
	return tmplCLI.Execute(w, primeData{SessionID: sessionID})
}

func outputTeamLeadContext(w io.Writer, sessionID string) error {
	if err := outputCLIContext(w, sessionID); err != nil {
		return err
	}
	return tmplTeamLead.Execute(w, primeData{SessionID: sessionID})
}

func outputTeammateContext(w io.Writer, role string, sessionID string) error {
	return tmplTeammate.Execute(w, primeData{SessionID: sessionID, Role: role})
}
```

Update the call sites in the `Run` function:

```go
		if role == "lead" {
			if err := outputTeamLeadContext(os.Stdout, sessionID); err != nil {
				os.Exit(0)
			}
			return
		} else if role != "" {
			if err := outputTeammateContext(os.Stdout, role, sessionID); err != nil {
				os.Exit(0)
			}
			return
		}

		// ...
		if err := outputPrimeContext(os.Stdout, mcpMode, sessionID); err != nil {
			os.Exit(0)
		}
```

- [ ] **Step 5: Verify build**

```bash
go build ./cmd/arc/...
```

- [ ] **Step 6: Write test for parseHookInput**

Create `cmd/arc/prime_test.go`. Test `parseHookInput` (the testable core) by passing `strings.NewReader(...)`:

```go
func TestParseHookInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid UUID", `{"session_id":"983a7cf7-bcb6-48fc-b485-129b4f1aaa45"}`, "983a7cf7-bcb6-48fc-b485-129b4f1aaa45"},
		{"invalid JSON", `not json`, ""},
		{"missing session_id", `{"cwd":"/tmp"}`, ""},
		{"invalid UUID format", `{"session_id":"not-a-uuid"}`, ""},
		{"empty input", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseHookInput(strings.NewReader(tt.input))
			if got != tt.want {
				t.Errorf("parseHookInput() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

```bash
go test ./cmd/arc/... -run TestParseHookInput -v
```

- [ ] **Step 7: Write test for persistSessionID**

Test cases:
- Writes to new file
- Replaces existing ARC_SESSION_ID line (idempotency)
- Preserves other env vars in the file

```go
func TestPersistSessionID(t *testing.T) {
	t.Run("writes to new file", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "env.sh")
		t.Setenv("CLAUDE_ENV_FILE", f)
		persistSessionID("abc-123")
		got, _ := os.ReadFile(f)
		if !strings.Contains(string(got), "export ARC_SESSION_ID=abc-123") {
			t.Errorf("expected ARC_SESSION_ID in file, got: %s", got)
		}
	})

	t.Run("replaces existing line", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "env.sh")
		os.WriteFile(f, []byte("export ARC_SESSION_ID=old-id\nexport OTHER=val\n"), 0o600)
		t.Setenv("CLAUDE_ENV_FILE", f)
		persistSessionID("new-id")
		got, _ := os.ReadFile(f)
		content := string(got)
		if strings.Contains(content, "old-id") {
			t.Error("old session ID should have been replaced")
		}
		if !strings.Contains(content, "export ARC_SESSION_ID=new-id") {
			t.Error("new session ID should be present")
		}
		if !strings.Contains(content, "export OTHER=val") {
			t.Error("other env vars should be preserved")
		}
	})
}
```

- [ ] **Step 8: Commit**

```bash
git add cmd/arc/prime.go cmd/arc/prime_test.go
git commit -m "feat: read hook stdin in arc prime and persist session ID"
```

## Chunk 4: Integration Tests and Prime Output

### Task 10: Integration Test for --take

**Files:**
- Modify: `tests/integration/update_test.go`

- [ ] **Step 1: Add TestUpdateTake test**

```go
// TestUpdateTake verifies that --take sets ai_session_id and status to in_progress.
func TestUpdateTake(t *testing.T) {
	home := setupHome(t)

	arcCmdSuccess(t, home, "init", "update-take-proj", "--server", serverURL)

	createOut := arcCmdSuccess(t, home, "create", "Take test issue", "--type", "task", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	// Take with explicit session ID
	arcCmdSuccessWithEnv(t, home, []string{"ARC_SESSION_ID=test-session-abc123"}, "update", id, "--take", "--server", serverURL)

	showOut := arcCmdSuccess(t, home, "show", id, "--json", "--server", serverURL)
	var issue map[string]interface{}
	if err := json.Unmarshal([]byte(showOut), &issue); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if issue["status"] != "in_progress" {
		t.Errorf("expected status in_progress, got %v", issue["status"])
	}
	if issue["ai_session_id"] != "test-session-abc123" {
		t.Errorf("expected ai_session_id test-session-abc123, got %v", issue["ai_session_id"])
	}
}

// TestUpdateTakeNoSession verifies that --take fails without a session ID.
func TestUpdateTakeNoSession(t *testing.T) {
	home := setupHome(t)

	arcCmdSuccess(t, home, "init", "take-nosess-proj", "--server", serverURL)

	createOut := arcCmdSuccess(t, home, "create", "No session test", "--type", "task", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	// Should fail — no ARC_SESSION_ID and no --session-id
	_, err := arcCmdRun(t, home, "update", id, "--take", "--server", serverURL)
	if err == nil {
		t.Error("expected error when --take used without session ID")
	}
}
```

Note: If `arcCmdSuccessWithEnv` doesn't exist in helpers, add it — it's `arcCmdSuccess` but with extra env vars on the exec.Cmd.

- [ ] **Step 2: Add helper if needed**

If `arcCmdSuccessWithEnv` doesn't exist in `helpers_test.go`, add it. Note: must include `ARC_SERVER` to match the existing helper pattern:

```go
func arcCmdSuccessWithEnv(t *testing.T, home string, env []string, args ...string) string {
	t.Helper()
	baseEnv := append(os.Environ(), "HOME="+home, "ARC_SERVER="+serverURL)
	cmd := exec.Command(arcBinary, args...)
	cmd.Env = append(baseEnv, env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("arc %v failed: %v\noutput: %s", args, err, string(out))
	}
	return string(out)
}
```

Also add `arcCmdRun` if not present (returns output + error without fatal):

```go
func arcCmdRun(t *testing.T, home string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(arcBinary, args...)
	cmd.Env = append(os.Environ(), "HOME="+home, "ARC_SERVER="+serverURL)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
```

- [ ] **Step 3: Commit**

```bash
git add tests/integration/update_test.go tests/integration/helpers_test.go
git commit -m "test: add integration tests for --take flag"
```

### Task 11: Update `arc prime` Template Output

> **Dependency:** Task 9 must be completed first — templates use `primeData` struct and `.SessionID`.

**Files:**
- Modify: `cmd/arc/prime.go` (tmplCLI, tmplMCP, tmplTeammate, tmplTeamLead templates)

- [ ] **Step 1: Add `--take` to the prime CLI reference**

In the `tmplCLI` template, update the "Creating & Updating" section to include the new flags. After the `arc update <id> --assignee=username` line, add:

```
- ` + "`arc update <id> --take`" + ` - Take issue for current AI session (sets session ID + in_progress)
```

- [ ] **Step 2: Add session display to all templates**

Add to `tmplCLI` (after the context recovery line), `tmplMCP` (after "# Arc Issue Tracker Active"), and `tmplTeammate` (after "# Arc Teammate Context"):

```
{{- if .SessionID}}
> **Session**: ` + "`{{.SessionID}}`" + `
{{- end}}
```

Note: `tmplTeamLead` inherits from `outputCLIContext` so it gets session display automatically.

- [ ] **Step 3: Update "Starting work" workflow**

In the Common Workflows section, update "Starting work" to use `--take`:

```
**Starting work:**
` + "```bash" + `
arc ready           # Find available work
arc show <id>       # Review issue details
arc update <id> --take  # Take it (sets session ID + in_progress)
` + "```" + `
```

- [ ] **Step 4: Verify build**

```bash
go build ./cmd/arc/...
```

- [ ] **Step 5: Commit**

```bash
git add cmd/arc/prime.go
git commit -m "feat: update arc prime output with --take docs and session display"
```

### Task 12: Final Verification

- [ ] **Step 1: Full build**

```bash
make build
```

- [ ] **Step 2: Run all unit tests**

```bash
make test
```

- [ ] **Step 3: Verify generated code is in sync**

```bash
make gen
git diff --exit-code
```

Expected: No diff — generated code should already be up to date.

- [ ] **Step 4: Manual end-to-end smoke test**

```bash
# Start server
./bin/arc-server &

# Init a project
./bin/arc init test-session-proj

# Create an issue
./bin/arc create "Test session tracking" --type=task

# Take it with explicit session ID
./bin/arc update <id> --take --session-id "983a7cf7-bcb6-48fc-b485-129b4f1aaa45"

# Verify it shows up
./bin/arc show <id>
# Should show: AI Session: 983a7cf7-bcb6-48fc-b485-129b4f1aaa45

# Take via env var
ARC_SESSION_ID="new-session-id" ./bin/arc update <id> --take

# Kill server
kill %1
```

- [ ] **Step 5: Final commit if any fixups needed, then push**

```bash
git push -u origin feat/ai-session-tracking
```
