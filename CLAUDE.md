# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

**arc** is a central issue tracking server for AI-assisted coding workflows. Unlike the distributed `beads` project, this uses a central server architecture.

**Read [AGENTS.md](AGENTS.md)** - it contains the complete workflow for AI agents, including the mandatory "landing the plane" session completion protocol.

### Product Boundary

Arc is **agent working memory**. New features must serve AI agents during active coding sessions. The litmus test: "Does an agent use this mid-session to decide what to do next or track what it just did?"

Arc's scope:
- Agent discovers unblocked work to start a session
- Agent tracks micro-progress within a session
- Agent records remaining work before a session ends
- Agent coordinates with other agents on parallel tasks

Human reporting, cross-session planning, and team visibility are served by dedicated tools (Jira, Linear) and should be bridged via skills or MCP — not built into arc.

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
   - Uses sqlc for type-safe queries (159 generated queries)
   - **Exception**: `ListIssues` and `GetLabelsForIssues` use hand-built dynamic SQL because sqlc's `sqlc.slice` macro is incompatible with positional `sqlc.narg`/`sqlc.arg` params in the same query (slice expansion shifts placeholder offsets at runtime). Do NOT attempt to convert these to sqlc queries — this is a known sqlc limitation, not tech debt.

2. **API Layer** (`internal/api/`)
   - Echo framework for HTTP routing
   - REST endpoints for projects, issues, dependencies, labels, comments

3. **CLI Layer** (`cmd/arc/`)
   - Cobra commands (commands defined in main.go + separate files)
   - Config in `~/.config/arc/config.json`

### Data Flow

```
CLI (arc) → HTTP Client → REST API → Storage → SQLite
```

## Key Data Types

See `internal/types/types.go`:
- **Project**: Top-level container for issues
- **Issue**: Core work item (title, status, priority 0-4, type, labels)
- **Dependency**: Four types (blocks, related, parent-child, discovered-from)
- **Status**: Open, InProgress, Blocked, Deferred, Closed
- **IssueType**: Bug, Feature, Task, Epic, Chore

## Claude Integration

The project includes Claude Code integration:

- **`arc init`**: Creates project and sets up AGENTS.md
- **`arc prime`**: Outputs AI workflow context for hooks
- **`arc setup claude`**: Installs SessionStart/PreCompact hooks
- **`arc onboard`**: Gets project orientation

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

# context-mode — MANDATORY routing rules

You have context-mode MCP tools available. These rules are NOT optional — they protect your context window from flooding. A single unrouted command can dump 56 KB into context and waste the entire session.

## BLOCKED commands — do NOT attempt these

### curl / wget — BLOCKED
Any Bash command containing `curl` or `wget` is intercepted and replaced with an error message. Do NOT retry.
Instead use:
- `ctx_fetch_and_index(url, source)` to fetch and index web pages
- `ctx_execute(language: "javascript", code: "const r = await fetch(...)")` to run HTTP calls in sandbox

### Inline HTTP — BLOCKED
Any Bash command containing `fetch('http`, `requests.get(`, `requests.post(`, `http.get(`, or `http.request(` is intercepted and replaced with an error message. Do NOT retry with Bash.
Instead use:
- `ctx_execute(language, code)` to run HTTP calls in sandbox — only stdout enters context

### WebFetch — BLOCKED
WebFetch calls are denied entirely. The URL is extracted and you are told to use `ctx_fetch_and_index` instead.
Instead use:
- `ctx_fetch_and_index(url, source)` then `ctx_search(queries)` to query the indexed content

## REDIRECTED tools — use sandbox equivalents

### Bash (>20 lines output)
Bash is ONLY for: `git`, `mkdir`, `rm`, `mv`, `cd`, `ls`, `npm install`, `pip install`, and other short-output commands.
For everything else, use:
- `ctx_batch_execute(commands, queries)` — run multiple commands + search in ONE call
- `ctx_execute(language: "shell", code: "...")` — run in sandbox, only stdout enters context

### Read (for analysis)
If you are reading a file to **Edit** it → Read is correct (Edit needs content in context).
If you are reading to **analyze, explore, or summarize** → use `ctx_execute_file(path, language, code)` instead. Only your printed summary enters context. The raw file content stays in the sandbox.

### Grep (large results)
Grep results can flood context. Use `ctx_execute(language: "shell", code: "grep ...")` to run searches in sandbox. Only your printed summary enters context.

## Tool selection hierarchy

1. **GATHER**: `ctx_batch_execute(commands, queries)` — Primary tool. Runs all commands, auto-indexes output, returns search results. ONE call replaces 30+ individual calls.
2. **FOLLOW-UP**: `ctx_search(queries: ["q1", "q2", ...])` — Query indexed content. Pass ALL questions as array in ONE call.
3. **PROCESSING**: `ctx_execute(language, code)` | `ctx_execute_file(path, language, code)` — Sandbox execution. Only stdout enters context.
4. **WEB**: `ctx_fetch_and_index(url, source)` then `ctx_search(queries)` — Fetch, chunk, index, query. Raw HTML never enters context.
5. **INDEX**: `ctx_index(content, source)` — Store content in FTS5 knowledge base for later search.

## Subagent routing

When spawning subagents (Agent/Task tool), the routing block is automatically injected into their prompt. Bash-type subagents are upgraded to general-purpose so they have access to MCP tools. You do NOT need to manually instruct subagents about context-mode.

## Output constraints

- Keep responses under 500 words.
- Write artifacts (code, configs, PRDs) to FILES — never return them as inline text. Return only: file path + 1-line description.
- When indexing content, use descriptive source labels so others can `ctx_search(source: "label")` later.

## ctx commands

| Command | Action |
|---------|--------|
| `ctx stats` | Call the `ctx_stats` MCP tool and display the full output verbatim |
| `ctx doctor` | Call the `ctx_doctor` MCP tool, run the returned shell command, display as checklist |
| `ctx upgrade` | Call the `ctx_upgrade` MCP tool, run the returned shell command, display as checklist |
