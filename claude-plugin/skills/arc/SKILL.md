---
name: arc
description: General arc CLI reference and workflow context. Use when the user asks about arc commands, issue tracking workflows, when to use arc vs TodoWrite, or needs help with arc setup and configuration.
---

# Arc Issue Tracker

Track complex, multi-session work with a central issue tracking system.

## Setup

**For Claude Code users** (recommended):
1. Install the arc plugin (provides hooks, skills, agents)
2. Run `arc onboard` in any project - it will:
   - Resolve project from server-side path registration (primary mechanism)
   - Or detect from local project config (`~/.arc/projects/`)
   - Or prompt you to run `arc init` for new projects

**For non-Claude users**:
```bash
arc init                    # Initialize project
```

The plugin is the single source of truth for Claude integration. It provides:
- **SessionStart/PreCompact hooks** - runs `arc prime` automatically
- **Prompt configuration** - reminds Claude to run `arc onboard`
- **Skills and resources** - detailed guides and reference
- **Agents** - for bulk operations

## When to Use Arc vs TodoWrite

| Use Arc | Use TodoWrite |
|---------|---------------|
| Multi-session work | Single-session tasks |
| Complex dependencies | Linear task lists |
| Discovered work patterns | Simple checklists |
| Work needing audit trail | Quick, disposable lists |

**Rule of thumb**: When in doubt, prefer arc—persistence you don't need beats lost context.

**Deep dive**: Run `arc docs boundaries` for detailed decision criteria.

## Workflow Skills

Arc includes workflow skills that guide you through the development lifecycle with built-in process discipline.

| Skill | Purpose | Invoke when |
|-------|---------|-------------|
| `brainstorm` | Design discovery through Socratic dialogue | Starting new features or significant work |
| `plan` | Break design into implementation tasks | After brainstorm approves a design |
| `implement` | TDD execution via fresh subagents per task | Ready to implement planned tasks |
| `debug` | 4-phase root cause investigation | Encountering bugs or test failures |
| `verify` | Evidence-based completion gates | Before claiming any work is done |
| `review` | Code review dispatch and triage | After implementing a task |
| `finish` | Session completion protocol | Ending a work session |

### Pipeline

```
brainstorm → plan → implement (per task) → review → finish
                        ↕          ↕
                      debug      verify
```

### Execution Paths

After `plan`, choose:
- **Single-agent + subagents**: Invoke `implement`. Main agent orchestrates, subagents do TDD. Best for sequential tasks.
- **Agentic team**: Add `teammate:*` labels, invoke `arc team-deploy`. Best for parallel multi-role work.

## Quick Start

Run `arc onboard` at session start to get project context and available issues.

**Project Recovery**: If local project config is missing, `arc onboard` resolves the project via server-side path registration. The server is the source of truth for project-to-directory mappings.

## CLI Reference

Run `arc prime` for full workflow context, or `arc <command> --help` for specific commands.

**Essential commands:**
- `arc ready` - Find unblocked work
- `arc create` - Create issues
- `arc update` - Update status/fields
- `arc close` - Complete work
- `arc show` - View details
- `arc dep` - Manage dependencies
- `arc plan` - Manage plans (inline, shared)
- `arc which` - Show active project and resolution source
- `arc paths` - Manage workspace path registrations
- `arc project` - Manage projects (list, create, delete, rename, merge)
- `arc self update` - Update arc CLI to latest version
- `arc db backup` - Create database backup

## Deep Dive Documentation

**Two-step workflow:**
1. **Search** to find which topic has the info: `arc docs search "query"`
2. **Read** the full topic for details: `arc docs <topic>`

```bash
# Search returns [topic] in brackets - tells you where to look
arc docs search "create issue"
# Results show: [workflows] Discovery and Issue Creation...

# Then read that topic for full content
arc docs workflows
```

Fuzzy matching handles typos - "dependncy" finds "dependency" docs.

**Available topics** with `arc docs <topic>`:

| Command | Purpose |
|---------|---------|
| `arc docs boundaries` | When to use arc vs TodoWrite - decision matrix, integration patterns, common mistakes |
| `arc docs workflows` | Step-by-step checklists for session start, epic planning, side quests, handoff |
| `arc docs dependencies` | Dependency types (blocks, related, parent-child, discovered-from) and when to use each |
| `arc docs resumability` | Writing notes that survive compaction - templates and anti-patterns |
| `arc docs plans` | Plan patterns (inline, parent-epic, shared) with examples |
| `arc docs plugin` | Claude Code plugin and Codex CLI integration guide |

Run `arc docs` without a topic to see an overview.

## Agent Mode

For bulk operations (creating epics with tasks, batch updates), use the **arc-issue-tracker** agent via the Task tool. This runs arc commands without consuming main conversation context.

## Dependency Types

Arc supports four dependency types:

| Type | Purpose | Affects Ready? |
|------|---------|----------------|
| **blocks** | Hard blocker - B can't start until A complete | Yes |
| **related** | Soft link - informational only | No |
| **parent-child** | Epic/subtask hierarchy | Yes |
| **discovered-from** | Track provenance of discovered work | No |

**Deep dive**: Run `arc docs dependencies` for examples and patterns.

## Plans

Plans are stored centrally with status tracking (draft, approved, rejected) and optional issue binding.

**CLI commands:**

| Command | Purpose |
|---------|---------|
| `arc plan set <id> "steps..."` | Create or update a plan for an issue (defaults to draft) |
| `arc plan show <id>` | Show the plan for an issue |
| `arc plan list` | List all plans (use `--status` to filter by draft/approved/rejected) |

**Plan patterns:**

| Pattern | When to Use | How |
|---------|------------|-----|
| **Inline** | Single issue with clear steps | `arc plan set <id> "steps..."` |
| **Parent Epic** | Epic with children sharing a plan | Set plan on parent; children inherit |

Plans default to **draft** status. They can be approved or rejected via the web UI, which provides a split-pane editor for review. Plans are shown automatically in `arc show <id>`. Run `arc docs plans` for full details.

## Labels

Labels are global (shared across all projects) and support colors and descriptions. Use labels for cross-cutting categorization like `security`, `performance`, `tech-debt`.

## Session Protocol

**At session start:**
```bash
arc onboard  # Get context, recover project if needed
```

**Before ending any session:**
Invoke the `finish` skill — it handles capturing remaining work, quality gates, arc updates, commit, and push. Work is NOT done until `git push` succeeds.

**Writing notes for resumability:**
```bash
arc update <id> --stdin <<'EOF'
COMPLETED: X. IN PROGRESS: Y. NEXT: Z
EOF
```

**Deep dive**: Run `arc docs resumability` for templates.

## Common Workflows

### Starting Work
```bash
arc onboard                         # Get context (recovers project if needed)
arc ready                           # Find available work
arc show <id>                       # View details
arc update <id> --status in_progress  # Claim work
```

### Creating Issues
```bash
arc create "Title" -t task          # Create task
arc create "Epic title" -t epic     # Create epic
arc create "Subtask" --parent <epic-id>  # Create child issue
arc dep add child-id parent-id --type parent-child  # Or link existing issue to epic

# With multi-line description (use --stdin flag):
arc create "Title" -t task --stdin <<'EOF'
Description with context, acceptance criteria, etc.
EOF
```

### Completing Work
```bash
arc close <id> --reason "done"      # Complete issue
arc ready                           # See what unblocked
```

**Deep dive**: Run `arc docs workflows` for complete checklists.
