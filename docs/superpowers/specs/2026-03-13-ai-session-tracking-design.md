# AI Session Tracking Design

**Date:** 2026-03-13
**Branch:** `feat/ai-session-tracking`
**Status:** Approved

## Problem

When AI agents (Claude Code, Codex CLI) work on arc issues, there's no way to:
1. View the session transcript of the agent that worked on an issue — especially subagents whose output is not easily visible
2. Resume a session that was working on a specific issue (`claude -r <session_id>`)

## Solution

Add an `ai_session_id` field to issues that stores the Claude Code session UUID. Provide a `--take` flag on `arc update` that atomically sets the session ID and marks the issue as in-progress. Use Claude Code's hook system to make the session ID available via environment variable.

## Design Decisions

- **New field, not rename:** Add `ai_session_id` alongside existing `assignee` rather than renaming. Avoids a 38-file rename; `assignee` can be deprecated separately later.
- **`arc update --take` over `arc take`:** Keep as a flag on the existing update command to reduce code surface. AI usage is guided by skills/prime output so discoverability doesn't matter.
- **Env var with explicit override:** `--take` reads `$ARC_SESSION_ID` by default, `--session-id` flag overrides. Zero friction for the happy path, escape hatch for edge cases.
- **Session ID handling in `arc prime`:** Rather than inline shell or separate script, `arc prime` reads hook stdin JSON itself. No `jq` dependency, testable in Go, keeps plugin config simple.
- **Preserved on close:** `ai_session_id` is never cleared — serves as historical record of which session worked on an issue.
- **Each agent takes its own issue:** Parent sessions take epics/top-level issues; subagents take their assigned child issues with their own session IDs.

## Data Layer

### Migration `011_ai_session_id.sql`

```sql
ALTER TABLE issues ADD COLUMN ai_session_id TEXT;
```

No index needed — queries go issue → session (by issue ID), not session → issue.

### Schema (`db/schema.sql`)

Add `ai_session_id TEXT` to the `issues` table definition.

### sqlc Queries (`db/queries/issues.sql`)

- Add `ai_session_id` to `CreateIssue` INSERT column list
- Add new query `UpdateIssueAISessionID`:
  ```sql
  UPDATE issues SET ai_session_id = ?, updated_at = ? WHERE id = ?;
  ```
- `ListIssuesFiltered` and `GetIssue` use `SELECT *` so they pick up the new column automatically

### Types (`internal/types/types.go`)

Add to `Issue` struct:
```go
AISessionID string `json:"ai_session_id,omitempty"`
```

### Storage (`internal/storage/sqlite/issues.go`)

Handle `"ai_session_id"` key in the `UpdateIssue` switch statement, calling the new `UpdateIssueAISessionID` query.

## API Layer

### OpenAPI (`api/openapi.yaml`)

- Add `ai_session_id` to `IssueResponse`, `CreateIssueRequest`, and `UpdateIssueRequest` schemas
- Add `ai_session_id` query parameter to list/search endpoints for filtering

### Handlers (`internal/api/issues.go`)

- Add `AISessionID string` to `createIssueRequest`
- Add `AISessionID *string` to `updateIssueRequest`
- Handle `"ai_session_id"` in the update map
- Read `ai_session_id` query param for list filtering

### Filter types (`internal/types/types.go`)

- Add `AISessionID *string` to `IssueFilter` and `ReadyFilter`

## CLI Layer

### Update command (`cmd/arc/main.go`)

New flags on `updateCmd`:
- `--take` (bool): Take the issue for the current AI session
- `--session-id` (string): Explicit session ID override

When `--take` is set:
1. Resolve session ID: `--session-id` flag > `$ARC_SESSION_ID` > error with message "no session ID available — set ARC_SESSION_ID or pass --session-id"
2. Add `"ai_session_id": <resolved-id>` to updates map
3. Add `"status": "in_progress"` to updates map (unless `--status` was explicitly passed)

`--session-id` without `--take` is an error.

### Show command (`cmd/arc/main.go`)

Display `AI Session: <uuid>` when `ai_session_id` is set in the issue detail output.

## Hook Integration

### `arc prime` stdin handling (`cmd/arc/prime.go`)

On startup:
1. Check if stdin is a pipe (not TTY) via `os.Stdin.Stat()` — check for absence of `os.ModeCharDevice`
2. If piped, read stdin with short timeout, parse as JSON
3. Extract `session_id` from payload
4. If `$CLAUDE_ENV_FILE` is set and `session_id` is present, append `export ARC_SESSION_ID=<session_id>` to that file
5. If parsing fails or stdin is TTY, silently continue

### `arc prime` output

When session ID is available (from stdin parse or `$ARC_SESSION_ID` env var), include in output:
```
Session: 983a7cf7-bcb6-48fc-b485-129b4f1aaa45
```

### Plugin config (`claude-plugin/.claude-plugin/plugin.json`)

No changes — hooks remain `arc prime` for both SessionStart and PreCompact.

## Code Generation

After making changes, run `make gen` to regenerate:
- sqlc Go types and queries
- OpenAPI Go server code
- TypeScript API types (web UI gets `ai_session_id` field for free)

## Out of Scope

- Web UI session transcript viewer (reading `.jsonl` files from `~/.claude/`)
- Web UI display of `ai_session_id` in components (types will be generated, wiring is separate)
- Deprecating `assignee` field
- `arc prime` outputting session-specific context (e.g., "you are working on issue X")

## Flow Summary

```
Session starts
  → Claude Code fires SessionStart hook
  → Hook sends JSON with session_id on stdin to `arc prime`
  → `arc prime` parses stdin, writes ARC_SESSION_ID to CLAUDE_ENV_FILE
  → `arc prime` outputs context including session ID

AI takes an issue:
  arc update <issue-id> --take
  → reads $ARC_SESSION_ID from environment
  → sets ai_session_id + status=in_progress on the issue

Subagent takes a child issue:
  → subagent gets its own SessionStart hook with its own session_id
  → arc update <child-id> --take (uses subagent's ARC_SESSION_ID)

Later:
  → Web UI reads issue.ai_session_id
  → Navigates to ~/.claude/projects/<path>/<session_id>.jsonl for transcript
  → Human can run: claude -r <session_id> to resume
```
