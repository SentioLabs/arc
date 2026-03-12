# Arc project overview

## Purpose
Arc is a central issue tracking server for AI-assisted coding workflows. It provides a REST API, CLI, and an embedded web UI for managing issues across multiple workspaces, backed by SQLite.

## Tech stack
- Go (server + CLI)
- SQLite (storage)
- Echo (HTTP routing)
- sqlc (DB query generation)
- oapi-codegen (OpenAPI server code)
- SvelteKit + Vite + TypeScript (web UI)
- bun (frontend package manager)

## Architecture / key components
- Central server (default `localhost:7432`), multi-workspace, SQLite DB at `~/.arc/data.db`
- CLI (`arc`) for issue management and workspace setup
- Embedded web UI in the Go server
- Claude Code integration (hooks/skills/agents)

## Codebase structure
- `cmd/server/` server binary
- `cmd/arc/` CLI binary
- `internal/api/` REST API handlers
- `internal/storage/` storage interface
- `internal/storage/sqlite/` SQLite implementation (sqlc)
- `internal/client/` HTTP client for CLI
- `internal/types/` domain types
- `web/` SvelteKit frontend

## Key patterns
- Workspace isolation for multi-tenant support
- Consistent JSON request/response and error handling in API layer
- Cobra for CLI parsing
- CLI config stored in `~/.config/arc/config.json` and project-local `.arc.json`
