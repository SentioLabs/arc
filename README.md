# Arc AI Issue Tracker

A central issue tracking server for AI-assisted coding workflows
This is heavily inspired by the amazing work over at [beads](https://github.com/steveyegge/beads)

Key Differences:

- no per-repo daemons
- no git sync or integration
- uses a REST API + CLI.
- embeded svelte web ui

## Features

- **Central Server**: Single server managing multiple workspaces
- **Web UI**: Svelete client app embedded in go server
- **REST API**: Clean JSON API for all operations
- **Workspaces**: First-class workspace management (replaces per-repo concept)
- **Full Issue Tracking**: Create, update, close, dependencies, labels, comments
- **Plans**: Inline plans on issues, shared plans linkable to multiple issues
- **Ready Work**: Find issues with no blockers
- **Statistics**: Aggregate metrics per workspace

## Installation

### Quick Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/sentiolabs/arc/main/scripts/install.sh | bash
```

### Linux Packages

Download `.deb`, `.rpm`, or `.pkg.tar.zst` (Arch) from the [latest release](https://github.com/sentiolabs/arc/releases/latest).

```bash
# Debian/Ubuntu
sudo dpkg -i arc_*_linux_amd64.deb

# Fedora/RHEL
sudo rpm -i arc_*_linux_amd64.rpm

# Arch Linux
sudo pacman -U arc_*_linux_amd64.pkg.tar.zst
```

### From Source

```bash
git clone https://github.com/sentiolabs/arc
cd arc
make build
# Binary at ./bin/arc
```

## Quick Start

### Start the Server

```bash
# Start the daemon (background)
arc server start

# Or run in foreground
arc server start --foreground

# Custom options
arc server start --foreground --port 8080 --db /path/to/data.db
```

The server stores data in `~/.arc/data.db` by default.

### CLI Usage

#### Getting Started

```bash
# Initialize arc in your project directory (creates workspace + config)
cd your-project
arc init                        # Uses directory name as workspace
arc init my-project             # Custom workspace name
arc init --prefix mp            # Custom issue prefix (e.g., mp-a3f2)

# Check which workspace is active
arc which
```

#### Day-to-Day Workflow

```bash
# Find available work
arc ready                       # Issues with no blockers
arc blocked                     # Issues waiting on dependencies

# Create issues
arc create "Implement feature X" -p 1 -t feature
arc create "Fix bug Y" -p 0 -t bug

# View and update issues
arc show mp-abc123
arc list                        # All issues
arc list --status open --type bug
arc update mp-abc123 --status in_progress
arc update mp-abc123 --assignee alice

# Close issues
arc close mp-abc123 --reason "Fixed in commit abc"

# View statistics
arc stats
```

#### Dependencies

```bash
# Add a blocker: def456 depends on abc123
arc dep add mp-def456 mp-abc123

# Other dependency types
arc dep add child parent --type parent-child
arc dep add issue-a issue-b --type related
arc dep add side-quest origin --type discovered-from
```

#### Epic & Subtask Patterns

```bash
# Create an epic with child issues
arc create "Auth overhaul" -t epic
arc create "JWT tokens" -t task --parent mp-abc123
arc create "OAuth provider" -t task --parent mp-abc123
```

#### Plans

```bash
# Inline plan on a single issue
arc plan set mp-abc123 "Step 1: do X. Step 2: do Y."
arc plan set mp-abc123 --editor          # Open $EDITOR
arc plan show mp-abc123
arc plan history mp-abc123

# Shared plans (linkable to multiple issues)
arc plan create "Q1 Initiative"
arc plan create "Q1 Initiative" --editor
arc plan link plan.xxxxx mp-abc123 mp-def456
arc plan unlink plan.xxxxx mp-abc123
arc plan list
arc plan edit plan.xxxxx
arc plan delete plan.xxxxx
```

#### Documentation & Help

```bash
arc docs                        # Overview of all topics
arc docs search "dependencies"  # Fuzzy search across all docs
arc docs plans                  # Full topic on plans
arc quickstart                  # Quick start guide
arc self update                 # Update to latest version
```

#### Advanced: Workspace Management

```bash
# For multi-project setups or manual workspace control
arc workspace create my-project --path /path/to/project
arc workspace use my-project    # Set default workspace
arc workspace list
arc workspace delete ws-abc123
```

### Claude Code Integration

For AI-assisted workflows, arc provides a Claude Code plugin with hooks, skills, and agents.

**Option A: Install Plugin (Recommended)**

```bash
# In Claude Code, first add the marketplace
/plugin marketplace add sentiolabs/arc

# Then install the plugin
/plugin install arc

# Restart Claude Code
```

**Option B: CLI Hooks Only**

```bash
arc setup claude            # Global installation
arc setup claude --project  # Project-only installation
arc setup claude --check    # Verify installation
```

**What the Plugin Provides:**

| Component         | Benefit                                |
| ----------------- | -------------------------------------- |
| SessionStart Hook | Auto-runs `arc prime` on session start |
| PreCompact Hook   | Preserves context before compaction    |
| Prompt Config     | Reminds Claude to run `arc onboard`    |
| Skills            | Detailed guides for arc workflows      |
| Agent             | Bulk operations via Task tool          |

**Typical Setup:**

```bash
# 1. Start the server (if not running)
arc server start

# 2. Initialize workspace in your project
cd your-project
arc init

# 3. Install Claude integration (choose one)
/plugin marketplace add sentiolabs/arc && /plugin install arc  # Full plugin
# OR
arc setup claude                                                # Hooks only
```

### Codex CLI Integration

Codex CLI supports repo-scoped skills under `.codex/skills`. Arc provides a Codex skill bundle
and a setup command to install it into your repo.

```bash
# 1. Start the server (if not running)
arc server start

# 2. Initialize workspace in your project
cd your-project
arc init

# 3. Install Codex integration (repo-scoped skill)
arc setup codex
```

**Notes:**
- Codex CLI does not support lifecycle hooks. Run `arc onboard` at session start.
- Use `arc prime` after compaction or if workflow context is stale.

### API Examples

```bash
# Health check
curl http://localhost:7432/health

# Create workspace
curl -X POST http://localhost:7432/api/v1/workspaces \
  -H "Content-Type: application/json" \
  -d '{"name": "my-project", "prefix": "mp"}'

# List issues
curl http://localhost:7432/api/v1/workspaces/ws-abc123/issues

# Create issue
curl -X POST http://localhost:7432/api/v1/workspaces/ws-abc123/issues \
  -H "Content-Type: application/json" \
  -d '{"title": "New feature", "priority": 1, "issue_type": "feature"}'

# Get ready work
curl http://localhost:7432/api/v1/workspaces/ws-abc123/ready
```

## Architecture

```mermaid
flowchart TB
    subgraph server["Central Server"]
        subgraph workspaces["Workspaces"]
            ws1["Workspace A"]
            ws2["Workspace B"]
            ws3["Workspace C"]
        end
        db[("SQLite DB<br/>~/.arc/data.db")]
        workspaces --> db
    end

    api{{"REST API<br/>localhost:7432"}}
    server --> api

    subgraph clients["CLI Clients"]
        cli1["arc CLI<br/>(project-1)"]
        cli2["arc CLI<br/>(project-2)"]
        cli3["arc CLI<br/>(project-3)"]
    end

    api --> cli1
    api --> cli2
    api --> cli3
```

## Data Model

### Workspace

- ID (e.g., "ws-a1b2")
- Name, description, path
- Prefix for issue IDs

### Issue

- ID (e.g., "mp-abc123")
- Title, description
- Status: open, in_progress, blocked, deferred, closed
- Priority: 0 (critical) - 4 (backlog)
- Type: bug, feature, task, epic, chore
- Assignee, external reference

### Dependency Types

- `blocks`: Issue A blocks issue B (affects ready work)
- `parent-child`: Hierarchical relationship (affects ready work)
- `related`: Loose association
- `discovered-from`: Discovered during work on another issue

### Label

- Name, color, description
- Global scope (shared across workspaces)

### Comment

- Text with author
- Type: `comment` (regular) or `plan` (inline plan)

### Event

- Audit trail entries (status changes, field updates, etc.)

### Plan (Shared)

- ID (e.g., "plan.xxxxx"), title, content
- Scoped to workspace, linkable to multiple issues

## Configuration

Arc uses three configuration layers (highest priority wins):

1. **CLI config** (`~/.arc/cli-config.json`) — server URL, default workspace
2. **Project config** (`~/.arc/projects/<path>/config.json`) — per-project workspace binding
3. **Legacy** (`.arc.json` in project root) — auto-migrated to project config

```json
// ~/.arc/cli-config.json
{
  "server_url": "http://localhost:7432",
  "default_workspace": "ws-abc123"
}
```

Resolution priority: CLI flag > project config > legacy `.arc.json` > CLI config defaults.

## Development

```bash
# Build everything (frontend + binaries)
make build

# Build binaries only (faster)
make build-quick

# Run tests
make test

# Generate code (sqlc, OpenAPI, TypeScript)
make gen

# Docker
make docker-build
make docker-up
```

## API Reference

### Workspaces

- `GET /api/v1/workspaces` - List workspaces
- `POST /api/v1/workspaces` - Create workspace
- `GET /api/v1/workspaces/:id` - Get workspace
- `PUT /api/v1/workspaces/:id` - Update workspace
- `DELETE /api/v1/workspaces/:id` - Delete workspace
- `GET /api/v1/workspaces/:id/stats` - Get statistics

### Issues

- `GET /api/v1/workspaces/:ws/issues` - List issues
- `POST /api/v1/workspaces/:ws/issues` - Create issue
- `GET /api/v1/workspaces/:ws/issues/:id` - Get issue
- `PUT /api/v1/workspaces/:ws/issues/:id` - Update issue
- `DELETE /api/v1/workspaces/:ws/issues/:id` - Delete issue
- `POST /api/v1/workspaces/:ws/issues/:id/close` - Close issue
- `POST /api/v1/workspaces/:ws/issues/:id/reopen` - Reopen issue

### Ready Work & Blocked

- `GET /api/v1/workspaces/:ws/ready` - Ready issues
- `GET /api/v1/workspaces/:ws/blocked` - Blocked issues

### Dependencies

- `GET /api/v1/workspaces/:ws/issues/:id/deps` - Get dependencies
- `POST /api/v1/workspaces/:ws/issues/:id/deps` - Add dependency
- `DELETE /api/v1/workspaces/:ws/issues/:id/deps/:dep` - Remove dependency

### Labels (Global)

- `GET /api/v1/labels` - List labels
- `POST /api/v1/labels` - Create label
- `PUT /api/v1/labels/:name` - Update label
- `DELETE /api/v1/labels/:name` - Delete label

### Issue Labels

- `POST /api/v1/workspaces/:ws/issues/:id/labels` - Add label to issue
- `DELETE /api/v1/workspaces/:ws/issues/:id/labels/:label` - Remove label

### Comments

- `GET /api/v1/workspaces/:ws/issues/:id/comments` - Get comments
- `POST /api/v1/workspaces/:ws/issues/:id/comments` - Add comment
- `PUT /api/v1/workspaces/:ws/issues/:id/comments/:cid` - Update comment
- `DELETE /api/v1/workspaces/:ws/issues/:id/comments/:cid` - Delete comment

### Inline Plans

- `POST /api/v1/workspaces/:ws/issues/:id/plan` - Set inline plan
- `GET /api/v1/workspaces/:ws/issues/:id/plan` - Get inline plan
- `GET /api/v1/workspaces/:ws/issues/:id/plan/history` - Get plan history

### Shared Plans

- `GET /api/v1/workspaces/:ws/plans` - List shared plans
- `POST /api/v1/workspaces/:ws/plans` - Create shared plan
- `GET /api/v1/workspaces/:ws/plans/:pid` - Get shared plan
- `PUT /api/v1/workspaces/:ws/plans/:pid` - Update shared plan
- `DELETE /api/v1/workspaces/:ws/plans/:pid` - Delete shared plan
- `POST /api/v1/workspaces/:ws/plans/:pid/link` - Link issues to plan
- `DELETE /api/v1/workspaces/:ws/plans/:pid/link/:id` - Unlink issue from plan

### Events

- `GET /api/v1/workspaces/:ws/issues/:id/events` - Get audit events

## License

MIT
