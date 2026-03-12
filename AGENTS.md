# Agent Instructions

This document provides guidelines for AI agents working with the arc codebase.

## Project Overview

arc is a central issue tracking server for AI-assisted coding workflows. It provides:

- **REST API** - Central server at `localhost:7432`
- **CLI** (`arc`) - Command-line interface for issue management
- **SQLite Storage** - Project-isolated issue storage
- **Claude Integration** - Hooks and skills for Claude Code

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Central Server                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │  Project A  │  │  Project B  │  │  Project C  │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
│                         │                               │
│                    SQLite DB                            │
│              (~/.arc/data.db)                  │
└─────────────────────────────────────────────────────────┘
                          │
              REST API (localhost:7432)
                          │
        ┌─────────────────┼─────────────────┐
        │                 │                 │
   ┌────▼────┐      ┌────▼────┐      ┌────▼────┐
   │  CLI    │      │  CLI    │      │  CLI    │
   │(proj-1) │      │(proj-2) │      │(proj-3) │
   └─────────┘      └─────────┘      └─────────┘
```

## Development Commands

```bash
# Build everything
make build

# Build binaries only (faster)
make build-quick

# Run server
./bin/arc-server                # Default port 7432
./bin/arc-server -addr :8080    # Custom port

# Run tests
make test

# Format code
make fmt

# Docker
make docker-up
```

## Code Organization

- `cmd/server/` - Server binary
- `cmd/arc/` - CLI binary
- `internal/api/` - REST API handlers
- `internal/storage/` - Storage interface
- `internal/storage/sqlite/` - SQLite implementation
- `internal/client/` - HTTP client for CLI
- `internal/types/` - Domain types

## Key Patterns

### Storage Layer
- Interface in `storage/storage.go`
- SQLite implementation with sqlc-generated queries
- Project isolation for multi-tenant support

### API Layer
- Echo framework for HTTP routing
- JSON request/response format
- Consistent error handling

### CLI Layer
- Cobra for command parsing
- Config stored in `~/.config/arc/config.json`
- Project config in `~/.arc/projects/<path>/config.json`

## Testing

```bash
# Run all tests
go test ./...

# Test specific package
go test ./internal/storage/sqlite/...

# With coverage
go test -cover ./...
```

## Commit Messages

Use Conventional Commits so tooling like goreleaser can generate changelogs:

- `feat: add project recovery`
- `fix: handle missing config`
- `chore: update deps`
- `docs: clarify setup`
- `refactor: simplify setup flow`

## Arc Issue Tracking

Use `arc` CLI for issue management. Run `arc prime` for full workflow context.

**Essential commands:**
```bash
arc ready                              # Find unblocked work
arc create "title" --type=task -p 2    # Create issue
arc update <id> --status in_progress   # Claim work
arc close <id>                         # Complete work
arc show <id>                          # View details
arc dep add <issue> <depends-on>       # Add dependency
```

### Agent Mode

For bulk operations (creating epics with tasks, batch updates), use the **arc-issue-tracker** agent via the Task tool. This runs arc commands in parallel without consuming main conversation context. The `plan` and `brainstorm` skills automatically delegate issue creation to this agent via structured manifests.

Example: "Create an epic for auth system with login and logout tasks"
→ Delegate to arc-issue-tracker agent

### Documentation Lookup

**Two-step workflow:**
1. **Search** to find which topic has the info: `arc docs search "query"`
2. **Read** the full topic for details: `arc docs <topic>`

Search results show `[topic]` in brackets - use that with `arc docs <topic>` for full content.

```bash
# Search returns topic name in brackets
arc docs search "create issue"
# Results: [workflows] Discovery and Issue Creation...

# Read that topic for full details
arc docs workflows
```

Fuzzy matching handles typos - "dependncy" finds "dependency" docs.

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Commit and push**:
   ```bash
   git add .
   git commit -m "description of changes"
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds

## Worktree Safety

Git worktrees (`isolation: "worktree"`) branch from HEAD at creation time. If sequential agents have committed work that moves HEAD, worktrees created afterward start from the updated HEAD — but worktrees created *before* those commits have a stale baseline. This means parallel worktree agents can miss prior sequential work entirely.

**Prevention:**
- Always commit and push all sequential work before creating worktrees
- Record `PARALLEL_BASE=$(git rev-parse HEAD)` before dispatching parallel agents
- Dispatch all parallel worktree agents in the same orchestrator turn (same message)
- After parallel agents merge back, verify sequential commits are still in `git log`
- Run the full test suite after any worktree merge

**Recovery** (if commits are missing after merge):
```bash
git reflog                            # Find the pre-merge state
git log --oneline <reflog-ref>        # Verify it has the missing commits
git cherry-pick <missing-commits>     # Restore them — or reset and re-merge
```

See `claude-plugin/skills/implement/SKILL.md` for the full Parallel Dispatch Protocol.
