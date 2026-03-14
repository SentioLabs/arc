# Project-Local Workspace Resolution

**Date:** 2026-02-27
**Status:** Approved

## Problem

`arc init` writes `.arc.json` to the project root containing a server-generated `workspace_id`. This ID is specific to the machine's `~/.arc/data.db`. When the same repo is used on multiple machines (e.g., macbook + linux desktop), `.arc.json` points to a workspace that doesn't exist on the other machine's database, breaking all arc commands.

## Solution

Move workspace binding out of the repo and into a per-machine project directory under `~/.arc/projects/`. Each machine maintains its own mapping from project path to workspace ID. No shared state between machines.

## Design

### Project Directory Layout

```
~/.arc/
├── data.db              # SQLite database (unchanged)
├── server.pid
├── server.log
├── cli-config.json      # CLI config (server URL)
└── projects/
    ├── -home-bfirestone-devspace-personal-sentiolabs-arc/
    │   └── config.json
    ├── -home-bfirestone-devspace-bactrack-bacstack/
    │   └── config.json
    └── ...
```

Directory names use the full absolute path with `/` replaced by `-` (following the Claude Code `~/.claude/projects/` convention).

**`config.json` contents:**
```json
{
  "workspace_id": "ws-05cgmu",
  "workspace_name": "arc",
  "project_root": "/home/bfirestone/devspace/personal/sentiolabs/arc"
}
```

`project_root` is stored for readability and validation.

### Workspace Resolution

When any `arc` command runs (e.g., `arc list` from a nested subdirectory):

**1. Git walk (primary):** Walk up from cwd looking for `.git/`. Use that directory as the project root. Convert to project dir name and look up `~/.arc/projects/<path>/config.json`.

**2. Prefix walk (secondary):** If no `.git/` found, convert cwd to the project dir format (`/` → `-`). Strip trailing segments longest-to-shortest, checking for a matching directory under `~/.arc/projects/` at each step.

Example for cwd `/home/user/project/foo/bar/baz`:
1. Check `-home-user-project-foo-bar-baz` — no match
2. Check `-home-user-project-foo-bar` — no match
3. Check `-home-user-project-foo` — no match
4. Check `-home-user-project` — match, use this

Longest match wins (most specific project).

**3. Legacy fallback:** Check for `.arc.json` in cwd and parent dirs. If found, migrate it (see Migration section).

### `arc init` Changes

- **Old:** Creates `.arc.json` in the project root
- **New:** Creates `~/.arc/projects/<path>/config.json`
- No longer writes any file to the repo directory (except AGENTS.md if opted in)
- Everything else (workspace creation on server, prefix generation) stays the same

### Migration Path

When the legacy fallback finds a `.arc.json`:
1. Read `workspace_id` and `workspace_name`
2. Create `~/.arc/projects/<path>/config.json` with those values
3. Print: `Migrated .arc.json → ~/.arc/projects/<path>/config.json`
4. Do **not** auto-delete `.arc.json` (may be committed to git — let the user clean it up)

## What This Does NOT Change

- Server architecture (still centralized SQLite)
- Database location (`~/.arc/data.db` by default)
- API endpoints or data model
- Workspace creation or management on the server side

## Future Work (Separate Features)

- **Configurable DB path:** Add `~/.config/arc/server.json` with a `db_path` field so users can point the server at a Syncthing-shared directory. Orthogonal to this change.
- **Sync safety guardrails:** WAL checkpoint on shutdown, integrity check on startup, Syncthing conflict file detection. Relevant when DB path is configurable.
