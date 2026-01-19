# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

**arc** is a central issue tracking server for AI-assisted coding workflows. Unlike the distributed `beads` project, this uses a central server architecture.

**Read [AGENTS.md](AGENTS.md)** - it contains the complete workflow for AI agents, including the mandatory "landing the plane" session completion protocol.

## Build & Test Commands

```bash
# Build binaries
go build -o arc-server ./cmd/server
go build -o arc ./cmd/bd

# Run server
./arc-server

# Run tests
go test ./...

# Generate sqlc code (after changing queries)
sqlc generate
```

## Architecture

### Three-Layer Design

1. **Storage Layer** (`internal/storage/`)
   - Interface in `storage.go`, SQLite implementation in `sqlite/`
   - Uses sqlc for type-safe queries

2. **API Layer** (`internal/api/`)
   - Echo framework for HTTP routing
   - REST endpoints for workspaces, issues, dependencies, labels, comments

3. **CLI Layer** (`cmd/arc/`)
   - Cobra commands (commands defined in main.go + separate files)
   - Config in `~/.config/arc/config.json`

### Data Flow

```
CLI (bd) → HTTP Client → REST API → Storage → SQLite
```

## Key Data Types

See `internal/types/types.go`:
- **Workspace**: Top-level container for issues
- **Issue**: Core work item (title, status, priority 0-4, type, labels)
- **Dependency**: Four types (blocks, related, parent-child, discovered-from)
- **Status**: Open, InProgress, Blocked, Deferred, Closed
- **IssueType**: Bug, Feature, Task, Epic, Chore

## Claude Integration

The project includes Claude Code integration:

- **`arc init`**: Creates workspace and sets up AGENTS.md
- **`arc prime`**: Outputs AI workflow context for hooks
- **`arc setup claude`**: Installs SessionStart/PreCompact hooks
- **`arc onboard`**: Gets workspace orientation

### Hooks

Install with `arc setup claude`:
- **SessionStart**: Runs `arc prime` when session starts
- **PreCompact**: Runs `arc prime` before context compaction

### Plugin

The `claude-plugin/` directory contains a Claude Code plugin:
- `plugin.json`: Plugin manifest with hooks
- `skills/arc/SKILL.md`: Skill documentation

## Session Completion (Mandatory)

Before ending work, complete ALL steps:

```bash
git status              # Check changes
git add <files>         # Stage code
git commit -m "..."     # Commit
git push                # Push - NOT DONE UNTIL THIS SUCCEEDS
```

## Key Documentation

- **[AGENTS.md](AGENTS.md)** - Complete AI workflow
- **[README.md](README.md)** - Project overview
