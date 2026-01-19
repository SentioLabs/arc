# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

**beads-central** is a central issue tracking server for AI-assisted coding workflows. Unlike the distributed `beads` project, this uses a central server architecture.

**Read [AGENTS.md](AGENTS.md)** - it contains the complete workflow for AI agents, including the mandatory "landing the plane" session completion protocol.

## Build & Test Commands

```bash
# Build binaries
go build -o beads-server ./cmd/server
go build -o bd ./cmd/bd

# Run server
./beads-server

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

3. **CLI Layer** (`cmd/bd/`)
   - Cobra commands (commands defined in main.go + separate files)
   - Config in `~/.config/beads-central/config.json`

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

- **`bd init`**: Creates workspace and sets up AGENTS.md
- **`bd prime`**: Outputs AI workflow context for hooks
- **`bd setup claude`**: Installs SessionStart/PreCompact hooks
- **`bd onboard`**: Gets workspace orientation

### Hooks

Install with `bd setup claude`:
- **SessionStart**: Runs `bd prime` when session starts
- **PreCompact**: Runs `bd prime` before context compaction

### Plugin

The `claude-plugin/` directory contains a Claude Code plugin:
- `plugin.json`: Plugin manifest with hooks
- `skills/beads/SKILL.md`: Skill documentation

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
