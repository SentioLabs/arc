# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

**arc** is a central issue tracking server for AI-assisted coding workflows. Unlike the distributed `beads` project, this uses a central server architecture.

**Read [AGENTS.md](AGENTS.md)** - it contains the complete workflow for AI agents, including the mandatory "landing the plane" session completion protocol.

## Build & Test Commands

```bash
# Build everything (frontend + binaries)
make build

# Build binaries only (faster, skips frontend)
make build-quick

# Run server
./bin/arc-server

# Run tests
make test

# Generate code (OpenAPI, sqlc, TypeScript types)
make gen

# Docker
make docker-build
make docker-up
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
CLI (arc) → HTTP Client → REST API → Storage → SQLite
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
- `skills/arc/SKILL.md`: Skill documentation (points to `arc prime`)
- `agents/arc-issue-tracker.md`: Agent for bulk operations

### Agent Mode

For bulk issue operations (creating epics with tasks, batch updates), use the **arc-issue-tracker** agent via the Task tool. This runs arc commands in parallel without consuming main conversation context.

### Svelte MCP

You are able to use the Svelte MCP server, where you have access to comprehensive Svelte 5 and SvelteKit documentation. Here's how to use the available tools effectively:
Available Svelte MCP Tools:
1. list-sections

Use this FIRST to discover all available documentation sections. Returns a structured list with titles, use_cases, and paths. When asked about Svelte or SvelteKit topics, ALWAYS use this tool at the start of the chat to find relevant sections.

2. get-documentation

Retrieves full documentation content for specific sections. Accepts single or multiple sections. After calling the list-sections tool, you MUST analyze the returned documentation sections (especially the use_cases field) and then use the get-documentation tool to fetch ALL documentation sections that are relevant for the user's task.

3. svelte-autofixer

Analyzes Svelte code and returns issues and suggestions. You MUST use this tool whenever writing Svelte code before sending it to the user. Keep calling it until no issues or suggestions are returned.

4. playground-link

Generates a Svelte Playground link with the provided code. After completing the code, ask the user if they want a playground link. Only call this tool after user confirmation and NEVER if code was written to files in their project.


## Session Completion

See **[AGENTS.md § Landing the Plane](AGENTS.md#landing-the-plane-session-completion)** for the mandatory session completion workflow.

## Key Documentation

- **[AGENTS.md](AGENTS.md)** - Complete AI workflow
- **[README.md](README.md)** - Project overview
