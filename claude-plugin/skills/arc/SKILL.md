# Arc Issue Tracker

Track complex, multi-session work with a central issue tracking system.

## Setup

**For Claude Code users** (recommended):
1. Install the arc plugin (provides hooks, skills, agents)
2. Run `arc onboard` in any project - it will:
   - Detect existing workspace from `.arc.json`
   - Or recover workspace from server if `.arc.json` is missing
   - Or prompt you to run `arc init` for new projects

**For non-Claude users**:
```bash
arc init                    # Initialize workspace
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

## Quick Start

Run `arc onboard` at session start to get workspace context and available issues.

**Workspace Recovery**: If `.arc.json` is missing but the workspace exists on the server (by directory path), `arc onboard` will automatically restore the local configuration. The server is the source of truth.

## CLI Reference

Run `arc prime` for full workflow context, or `arc <command> --help` for specific commands.

**Essential commands:**
- `arc ready` - Find unblocked work
- `arc create` - Create issues
- `arc update` - Update status/fields
- `arc close` - Complete work
- `arc show` - View details
- `arc dep` - Manage dependencies

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

Run `arc docs` without a topic to see an overview.

## Agent Mode

For bulk operations (creating epics with tasks, batch updates), use the **arc-issue-tracker** agent via the Task tool. This runs arc commands without consuming main conversation context.

## Dependency Types

Arc supports four dependency types:

| Type | Purpose | Affects Ready? |
|------|---------|----------------|
| **blocks** | Hard blocker - B can't start until A complete | Yes |
| **related** | Soft link - informational only | No |
| **parent-child** | Epic/subtask hierarchy | No |
| **discovered-from** | Track provenance of discovered work | No |

**Deep dive**: Run `arc docs dependencies` for examples and patterns.

## Session Protocol

**At session start:**
```bash
arc onboard  # Get context, recover workspace if needed
```

**Before ending any session:**
Follow the **Landing the Plane** protocol in `AGENTS.md`. This includes:
- Filing issues for remaining work
- Running quality gates (tests, linters)
- Committing and pushing (work is NOT done until `git push` succeeds)
- Handing off context for the next session

**Writing notes for resumability:**
```bash
arc update <id> --description "COMPLETED: X. IN PROGRESS: Y. NEXT: Z"
```

**Deep dive**: Run `arc docs resumability` for templates.

## Common Workflows

### Starting Work
```bash
arc onboard                         # Get context (recovers workspace if needed)
arc ready                           # Find available work
arc show <id>                       # View details
arc update <id> --status in_progress  # Claim work
```

### Creating Issues
```bash
arc create "Title" -t task          # Create task
arc create "Epic title" -t epic     # Create epic
arc dep add child-id parent-id --type parent-child  # Link to epic
```

### Completing Work
```bash
arc close <id> --reason "done"      # Complete issue
arc ready                           # See what unblocked
```

**Deep dive**: Run `arc docs workflows` for complete checklists.
