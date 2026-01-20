# Agent Instructions

This document provides guidelines for AI agents working with the arc codebase.

## Project Overview

arc is a central issue tracking server for AI-assisted coding workflows. It provides:

- **REST API** - Central server at `localhost:7432`
- **CLI** (`arc`) - Command-line interface for issue management
- **SQLite Storage** - Workspace-isolated issue storage
- **Claude Integration** - Hooks and skills for Claude Code

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Central Server                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │ Workspace A │  │ Workspace B │  │ Workspace C │     │
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
- Workspace isolation for multi-tenant support

### API Layer
- Echo framework for HTTP routing
- JSON request/response format
- Consistent error handling

### CLI Layer
- Cobra for command parsing
- Config stored in `~/.config/arc/config.json`
- Project-local config in `.arc.json`

## Testing

```bash
# Run all tests
go test ./...

# Test specific package
go test ./internal/storage/sqlite/...

# With coverage
go test -cover ./...
```

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

For bulk operations (creating epics with tasks, batch updates), use the **arc-issue-tracker** agent via the Task tool. This runs arc commands in parallel without consuming main conversation context.

Example: "Create an epic for auth system with login and logout tasks"
→ Delegate to arc-issue-tracker agent

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
