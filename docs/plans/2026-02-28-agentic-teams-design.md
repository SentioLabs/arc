# Agentic Teams Integration Design

**Date:** 2026-02-28
**Status:** Approved

## Problem

Arc has labels (global, with colors), plans (inline + shared), assignees, and a dependency graph. Claude Code has experimental agent teams (TeamCreate, TaskList, SendMessage). These systems don't talk to each other. The result: agent teams reinvent work decomposition every session, arc issues go stale during team work, and there's no visibility into team activity from the web UI.

## Design Direction

**Arc as strategic context, Claude Code TaskList as tactical execution.**

Arc provides the persistent layer — labeled issues, plans, dependency graphs, audit trails. Claude Code's TaskList handles real-time coordination — claim, block, complete. The team lead bridges the two: reading arc to decompose work, and syncing results back after verification.

### Principles

- Don't replace Anthropic's TaskList — complement it
- Team lead owns arc sync (no teammate-driven churn)
- Labels drive teammate routing via `teammate:*` convention
- Plans serve dual purpose: team brief + acceptance criteria

## Data Model

### `teammate:*` Label Convention

No schema changes. Labels with the `teammate:` prefix indicate teammate routing:

| Label | Meaning |
|---|---|
| `teammate:frontend` | Frontend — UI components, styling, client logic |
| `teammate:backend` | Backend — API, storage, business logic |
| `teammate:architect` | Architecture — design decisions, cross-cutting |
| `teammate:tests` | Tests — coverage, verification |
| `teammate:devops` | DevOps — CI/CD, infrastructure, deployment |

Users create these via the existing `/labels` page or future `arc label` CLI commands. Colors flow through to the web Team View.

Rules:
- An issue can have multiple `teammate:*` labels (the orchestration skill assigns to the primary or creates subtasks)
- Regular labels (`bug`, `feature`, `P1`) coexist as metadata, not routing

### Plan Integration

- **Shared plans** linked to an epic = team brief (read at decomposition time)
- **Inline plans** on individual issues = acceptance criteria (read at verification time)
- No changes to plan data model

## Component 1: Enhanced `arc prime`

### Teammate Detection

When the orchestration skill spawns a teammate, it sets:
```
ARC_TEAMMATE_ROLE=frontend
```

`arc prime` checks this env var (or a `--role` flag) to determine output mode.

### Output Modes

**Default mode** (no role set) — unchanged. CLI reference + session close protocol.

**Team lead mode** (`ARC_TEAMMATE_ROLE=lead`):
- Full CLI reference
- Team orchestration guidance: task list usage, arc sync protocol
- Verification checklist: review teammate work before closing arc issues
- Session close protocol with team cleanup steps

**Teammate mode** (`ARC_TEAMMATE_ROLE=frontend`, etc.):
- Filtered issue list: only issues with matching `teammate:*` label
- Each issue: title, status, priority, inline plan, linked shared plan summary
- Relevant dependencies
- Session close protocol (git commit/push)

### Implementation

- Modify `cmd/arc/prime.go`: add role detection (env var + `--role` flag)
- Add teammate-specific output template
- No new storage methods

## Component 2: `arc team context` CLI Command

Outputs structured data for the orchestration skill. Separates "gathering arc data" from "creating a Claude Code team."

### Signature

```bash
arc team context [epic-id] [flags]
```

**With `epic-id`:** Read epic's children, group by `teammate:*` labels, include shared plan.
**Without `epic-id`:** Read all ready issues with `teammate:*` labels in active workspace.

### Flags

| Flag | Description |
|---|---|
| `--json` | Machine-readable JSON (default for skill consumption) |
| `--format=human` | Human-readable summary (default for interactive use) |

### JSON Output Structure

```json
{
  "workspace": "my-project",
  "epic": { "id": "ISSUE-5", "title": "Implement auth system", "plan": "..." },
  "roles": {
    "frontend": {
      "issues": [
        { "id": "ISSUE-12", "title": "Login form", "priority": 2, "plan": "...", "deps": ["ISSUE-8"] }
      ]
    },
    "backend": {
      "issues": [
        { "id": "ISSUE-8", "title": "Auth API endpoints", "priority": 1, "plan": "..." }
      ]
    }
  },
  "unassigned": [
    { "id": "ISSUE-20", "title": "Write tests", "priority": 3 }
  ]
}
```

### Implementation

- New file: `cmd/arc/team.go`
- Calls existing storage methods: `ListIssues`, `GetIssueLabels`, `GetDependencies`, `GetPlan`
- No new storage methods — read-only aggregation

## Component 3: Team Orchestration Skill

A Claude Code skill the team lead invokes to decompose arc issues into a team.

### Location

`claude-plugin/skills/arc-team-deploy/SKILL.md`

### Workflow

**Step 1 — Discover work:**
```bash
arc team context <epic-id> --json
```

**Step 2 — Propose team composition:**
Present grouped issues to lead/user for approval:
```
Proposed team:
  - frontend (3 issues): ISSUE-12, ISSUE-14, ISSUE-16
  - backend (2 issues): ISSUE-8, ISSUE-11
  - architect (1 issue): ISSUE-7

Plan brief: [summary of shared plan]
Unassigned: ISSUE-20 (no teammate:* label)
```

**Step 3 — Create team + tasks:**
After approval:
1. `TeamCreate` with team name
2. `TaskCreate` per issue (subject = issue title, description = issue details + plan)
3. Task dependencies mirroring arc's dependency graph
4. Spawn teammates via `Agent` tool with `ARC_TEAMMATE_ROLE=<role>` env var
5. `TaskUpdate` to assign tasks to appropriate teammate

**Step 4 — Team lead sync protocol:**
1. Teammate marks TaskList item complete → lead reviews (or asks QA teammate to verify)
2. If satisfied → `arc close <id> --reason "completed by <teammate>"`
3. If not → reassign or provide feedback

### Invocation

```
/arc team-deploy
```

## Component 4: Web Team View

### Route

`/teams` (or `/workspace/:id/teams`)

### API Endpoint

```
GET /api/v1/workspaces/{ws}/team-context?epic_id=PROJ-5
```

Returns the same grouped structure as the CLI command's JSON output.

### Layout

```
┌─────────────────────────────────────────────────────────────────┐
│  TEAM DEPLOYMENT          Epic: PROJ-5 · Implement Auth System  │
│  Plan: "JWT-based auth with refresh tokens, httpOnly cookies…"  │
├──────────────┬──────────────┬──────────────┬────────────────────┤
│  ◈ ARCHITECT │  ◈ BACKEND   │  ◈ FRONTEND  │  ◈ UNASSIGNED      │
│  1 issue     │  2 issues    │  3 issues    │  1 issue           │
├──────────────┼──────────────┼──────────────┼────────────────────┤
│  [cards]     │  [cards]     │  [cards]     │  [cards]           │
└──────────────┴──────────────┴──────────────┴────────────────────┘
```

- **Role columns**: Each `teammate:*` label → column. Header shows role name + label color accent stripe.
- **Epic header**: Title + truncated plan summary. Expandable.
- **Issue cards**: Compact — ID, title, priority, type badge, status icon, dependency links.
- **Unassigned column**: Issues without `teammate:*` labels.
- **Read-only**: Changes happen through arc CLI.

### Components

| Component | Purpose |
|---|---|
| `TeamView.svelte` | Page layout — epic header + role columns |
| `RoleLane.svelte` | Single role column with header + issue cards |
| `TeamIssueCard.svelte` | Compact issue card for team context |

### Design

"Refined Terminal" aesthetic — dark oklch surfaces, electric indigo accents, JetBrains Mono + Instrument Sans. Mission control dashboard feel: tight, purposeful, information-dense.

## Component 5: Brainstorming/Planning → Arc Storage

### Current State

```
brainstorming skill → Write docs/plans/YYYY-MM-DD-<topic>-design.md
writing-plans skill → Write docs/plans/YYYY-MM-DD-<topic>.md (checklist)
executing-plans skill → TaskCreate items, works through them
```

Output is ephemeral markdown — useful within a session but disconnected from arc's persistent tracking.

### With Arc Integration

```
brainstorming skill → arc create "Topic" --type=epic
                    → arc plan set <epic-id> "design content"

writing-plans skill → arc create "Step 1" --parent=<epic-id> --type=task
                    → arc create "Step 2" --parent=<epic-id> --type=task
                    → (optionally) arc label add <id> teammate:frontend
                    → arc dep add <step-2> <step-1>  (if sequential)

executing-plans     → reads arc issues, works through them
    OR
team-deploy         → reads arc issues, deploys a team
```

### Skill Modifications

**Brainstorming skill** — change the "Write design doc" step:
- Instead of `Write` to `docs/plans/`, run:
  ```bash
  arc create "Design Topic" --type=epic --description="..."
  arc plan set <epic-id> --stdin  # pipe design content
  ```
- Design lives in arc as a shared plan on an epic

**Writing-plans skill** — change the output step:
- Instead of writing a markdown checklist, run:
  ```bash
  arc create "Step 1: Set up auth middleware" --type=task --parent=<epic-id> --priority=1
  arc create "Step 2: Implement login endpoint" --type=task --parent=<epic-id> --priority=2
  arc dep add <step-2-id> <step-1-id>
  ```
- Optionally prompt: "Should I add `teammate:*` labels for team deployment?"

**Executing-plans skill** — change the input step:
- Instead of reading a markdown file, run:
  ```bash
  arc list --parent=<epic-id> --json
  ```
- Works through arc issues instead of markdown checklist
- Calls `arc close <id>` as each step completes

### CLI Convenience

New `arc plan set` flag to accept piped content:
```bash
echo "design content" | arc plan set <epic-id> --stdin
```
Avoids `--editor` (interactive) when skills write programmatically.

### Future: Arc-Aware Skills (V2)

Add a "read context" input phase to each skill:
- Brainstorming checks for existing related issues/plans before proposing
- Planning reads dependency graphs to understand sequencing constraints
- Purely additive — no restructuring needed since data is already in arc

## Workflow Summary

### Full Pipeline (brainstorm → deploy)

```
/brainstorm "Build auth system"
  ↓
Brainstorming skill → arc create epic + arc plan set (design)
  ↓
/writing-plans
  ↓
Planning skill → arc create child issues with deps
              → optionally add teammate:* labels
  ↓
/arc team-deploy  (or /executing-plans for single-agent)
  ↓
Orchestration skill → arc team context <epic-id>
  ↓
Proposes team composition → user approves
  ↓
Creates Claude Code team + TaskList items
Spawns teammates with ARC_TEAMMATE_ROLE env var
  ↓
Teammates get role-filtered context via arc prime
Work using Claude Code TaskList for coordination
  ↓
Teammate completes TaskList item
  ↓
Team lead verifies (optionally via QA teammate)
  ↓
Team lead runs `arc close <id>` → arc issue closed
  ↓
Web Team View reflects status changes in real-time
```

### Standalone Team Deploy (issues already exist)

```
User labels issues with teammate:* labels
User links shared plan to epic
  ↓
/arc team-deploy
  ↓
(same flow as above from orchestration onward)
```

## What Doesn't Change

- Label schema (no new fields)
- Plan schema (no new fields)
- Issue schema (no new fields)
- Existing `arc prime` default output (non-team sessions unchanged)
- Claude Code's TaskList behavior
- The `arc-issue-tracker` agent (still useful for non-team bulk operations)

## Verification

```bash
# Go changes
make test

# Web changes
cd web && bun run lint && bun run format:check

# Manual verification
# 1. Create teammate:* labels via /labels page
# 2. Create epic with child issues, label them
# 3. Run `arc team context <epic-id>` — verify grouped output
# 4. Run `arc prime` with ARC_TEAMMATE_ROLE=frontend — verify filtered output
# 5. Visit /teams — verify dashboard renders correctly
```
