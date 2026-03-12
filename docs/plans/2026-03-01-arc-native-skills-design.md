# Arc-Native Skills Design

**Date:** 2026-03-01
**Status:** Approved

## Problem

Two competing workflow systems — arc's issue tracking + "Landing the Plane" session protocol, and claude-superpowers' skill pipeline (brainstorm → plan → implement → finish). Using both causes friction at every layer:

- **Workflow pipeline mismatch**: Superpowers' skill chain doesn't map to arc's issue lifecycle
- **Task tracking confusion**: TodoWrite vs arc issues — Claude uses them inconsistently
- **Context overload**: Both systems inject heavy context, Claude drops one mid-session
- **Conflicting session protocols**: "Landing the Plane" vs "finishing-a-development-branch"

## Design Direction

**Build arc-native skills that embed superpowers' process disciplines into arc's issue lifecycle.** No dependency on the superpowers repo. One plugin, one workflow, no conflicts.

### Principles

- Arc issues for persistent work items, TodoWrite for in-session step checklists
- Arc plans as the sole artifact (no `docs/plans/` markdown duplication)
- Main agent brainstorms/plans/orchestrates — subagents implement with fresh context per task
- Compose with the agentic teams design — same arc artifacts, different execution paths
- Embed superpowers' iron laws and hard gates directly in skill content

## Arc + TodoWrite Complementary Roles

| Concern | Arc Issues | TodoWrite |
|---------|-----------|-----------|
| **Scope** | Work items (features, bugs, tasks) | Steps within a work item |
| **Lifespan** | Multi-session, survives compaction | Single session, ephemeral |
| **Granularity** | "Implement auth middleware" | "Write failing test for token validation" |
| **Dependencies** | Between work items (blocks, parent-child) | Ordered checklist within one item |
| **Persistence** | Server-backed, queryable | Gone when session ends |
| **UI** | CLI output | Native progress display |

How they compose in practice:

1. `arc ready` → pick issue
2. `arc update <id> --status in_progress` → claim it
3. TodoWrite → create implementation steps for that issue
4. Work through TodoWrite checklist within the session
5. `arc close <id> --reason "done"` → mark complete

Arc never needs to know about TodoWrite steps. TodoWrite never needs to know about other issues or dependencies.

## Plan Hierarchy

### Two-Level Structure

```
Meta Epic: "Project X"                     ← arc plan = DESIGN PLAN
├── Epic: "Phase 1: Core Foundations"      ← arc plan = IMPLEMENTATION PLAN (phase 1)
│   ├── Task: Scaffold workspace
│   ├── Task: Hashing module
│   └── Task: ...
├── Epic: "Phase 2: Storage Engine"        ← arc plan = IMPLEMENTATION PLAN (phase 2)
│   ├── Task: Schema setup
│   └── Task: ...
└── Epic: "Phase 3: Server + Auth"         ← arc plan = IMPLEMENTATION PLAN (phase 3)
    └── ...
```

For smaller work the hierarchy collapses:

| Scale | Structure |
|-------|-----------|
| Large (multi-phase project) | Meta epic → phase epics → tasks |
| Medium (single feature) | Epic → tasks (no meta epic) |
| Small (single task) | One issue, no epic |

### Token Efficiency

At plan-creation time, the `plan` skill distributes content:

- **Phase epic plan** = full implementation plan (for recovery/reference)
- **Task description** = that task's specific section (files, steps, code)

At implementation time, load only what's needed:

- `arc show <task-id>` → ~3-5k (self-contained task section)
- NOT `arc plan show <phase-epic-id>` → 54-97k (entire phase plan)

This is ~15x context reduction per task vs loading the full plan every time.

### When to Load the Full Plan

| Situation | What to load |
|-----------|-------------|
| Implementing a task | `arc show <task-id>` only |
| Planning a phase | `arc plan show <meta-epic-id>` (design) |
| Recovering after compaction | `arc prime` → `arc show <epic-id>` |
| Debugging cross-task issues | `arc plan show <phase-epic-id>` (full plan) |

## Workflow Pipeline

### The Fork Point

```
brainstorm ──→ plan ──→ ┬──→ implement (single-agent + subagents)
                        │     Small/medium work, sequential tasks
                        │
                        └──→ arc team-deploy (agentic team)
                              Large parallel work, multi-role
```

Both paths consume the same arc artifacts: epic, plan, child issues, dependencies. The `plan` skill optionally adds `teammate:*` labels to prepare for team deployment.

### Agent Role Separation

The main agent never writes implementation code. Its job:

| Phase | Main agent does | Main agent does NOT do |
|-------|----------------|----------------------|
| Brainstorm | Explores, asks questions, designs | — |
| Plan | Creates task breakdown, writes arc issues | Write implementation code |
| Implement | Dispatches subagent, reviews result | Touch source files |
| Review | Dispatches reviewer, triages feedback | Implement fixes (re-dispatches) |
| Finish | Quality gates, arc updates, commit, push | — |

### Context Flow

```
Main agent:  brainstorm ──→ plan ──→ orchestrate loop ──→ finish
                                          │
                              ┌───────────┼───────────┐
                              ▼           ▼           ▼
                         implement    implement    implement
                         (subagent)   (subagent)   (subagent)
                              │           │           │
                              ▼           ▼           ▼
                          review      review      review
                         (subagent)   (subagent)   (subagent)
```

Each subagent starts fresh with ~4k of context from arc, leaving the full context budget for actual coding.

## Skill Inventory

### 1. `brainstorm` — Socratic Requirements Discovery

- Explores project context, asks questions one at a time, proposes 2-3 approaches
- **Hard gate**: No code before design approval
- **Arc integration**: Creates epic + saves design as arc plan
- **TodoWrite**: Tracks the brainstorming checklist (explore → questions → approaches → design)
- **Output**: Arc epic with design plan
- **Scale detection**: Creates meta epic + phase epics for large work, single epic for medium, single issue for small
- **Next**: `plan`

### 2. `plan` — Implementation Task Breakdown

- Takes approved design and breaks it into bite-sized tasks with exact file paths and steps
- **Granularity rule**: Each step is ONE action, 2-5 minutes, exact file paths
- **Assumption**: Implementer has zero codebase context
- **Arc integration**: Creates child task issues with dependencies, updates epic's arc plan with implementation breakdown, copies each task's section into the task's description (for token efficiency)
- **TodoWrite**: Tracks planning steps
- **Team preparation**: After creating issues, asks whether this will be single-agent or team. If team, prompts for `teammate:*` labels per task
- **Output**: Arc issues with self-contained descriptions, epic plan updated
- **Next**: `implement` (single-agent) or `arc team-deploy` (agentic team)

### 3. `implement` — TDD-Driven Task Execution

- Main agent picks task from `arc ready`, dispatches `arc-implementer` subagent
- **Iron Law**: NO PRODUCTION CODE WITHOUT FAILING TEST FIRST
- **TDD cycle**: RED (write failing test, watch it fail) → GREEN (simplest code to pass) → REFACTOR (clean up, tests stay green)
- **Rationalizations countered**: "too simple to test", "I'll test after", "manual testing enough"
- **Arc integration**: `arc update <id> --status in_progress` at dispatch, `arc close <id>` after verification
- **Subagent receives**: Task description from `arc show <task-id>`, TDD discipline in agent prompt
- **Next**: `review` (after task), `debug` (if something breaks)

### 4. `debug` — 4-Phase Systematic Debugging

- Invoked when something breaks during implementation (or standalone)
- **Iron Law**: NO FIXES WITHOUT ROOT CAUSE INVESTIGATION FIRST
- **4 phases**: Investigate root cause → Pattern analysis → Hypothesis test → Implement fix
- **3-fix rule**: 3+ failed fixes = question the architecture, don't try fix #4
- **Arc integration**: Can create a new bug issue if the problem is bigger than expected
- **TodoWrite**: Tracks the 4 phases
- **Next**: Returns to `implement` (or wherever you were)

### 5. `verify` — Evidence-Based Completion Gates

- Run before claiming any work is done
- **Iron Law**: NO COMPLETION CLAIMS WITHOUT FRESH VERIFICATION EVIDENCE
- **Gate sequence**: Identify proof command → Run it → Read full output → Verify claim
- **Red flags**: "should work", "probably fine", expressing satisfaction before running tests
- **Arc integration**: Only `arc close` after verification passes
- **TodoWrite**: Tracks verification steps
- **Next**: `finish` (session end) or back to `implement` (if verification fails)

### 6. `review` — Code Review Dispatch

- Dispatches `arc-reviewer` subagent to review work
- **Dispatch**: Gets git SHAs, sends diff + task spec to reviewer
- **Response discipline**: Technical evaluation, not performative agreement
- **Triage**: Critical → fix immediately, Important → fix before proceeding, Minor → note for later
- **Arc integration**: Reviews reference the arc issue being implemented
- **Works in both contexts**: Subagent dispatch (single-agent) or QA teammate (team mode)
- **Next**: Back to `implement` (if fixes needed), `finish` (if clean)

### 7. `finish` — Unified Session Completion

Replaces both "Landing the Plane" and "finishing-a-development-branch". One protocol.

**Phase 1: Capture Remaining Work**
- Review planned vs completed
- `arc create` for unfinished or discovered work
- Add context notes to new issues

**Phase 2: Quality Gates** (if code changed)
- Run test suite, linter/formatter, build
- **Hard gate**: If tests fail, fix before proceeding

**Phase 3: Update Arc Issues**
- `arc close <id> -m "reason"` for completed work
- `arc update <id> --description "progress notes"` for in-progress work

**Phase 4: Commit and Push**
- `git add` specific files
- `git commit` with conventional commit message
- `git push`
- `git status` — must show "up to date with origin"
- **Iron Law**: Work is NOT done until push succeeds

**Phase 5: Verify and Hand Off**
- `git log -1` — confirm latest commit
- `arc prime` — output context for next session

**Context-aware behavior**:

| Context | Behavior |
|---------|----------|
| Single-agent session | Full protocol above |
| Team lead session | Verify teammate work → arc close → team cleanup → commit → push |
| Teammate session | Commit → push (team lead handles arc close) |

## Agents

### `arc-implementer`

Subagent dispatched per task. Fresh context.

**Receives**: Task description from `arc show <task-id>`, TDD discipline in prompt

**Does**:
1. Reads task description (files, steps, code guidance)
2. Follows RED → GREEN → REFACTOR cycle
3. Runs tests, verifies they pass
4. Commits the work
5. Reports result back to main agent

**Does NOT**: Interact with the user, manage arc issues, review its own work

### `arc-reviewer`

Subagent dispatched for code review. Fresh context.

**Receives**: Git diff, task description for spec compliance

**Does**:
1. Reviews changes against task spec
2. Reviews against project conventions
3. Categorizes: Critical / Important / Minor
4. Reports findings to main agent

**Does NOT**: Make code changes, close issues

## Discipline Content Embedded From Superpowers

Each skill embeds the relevant iron laws and hard gates directly. No dependency on the superpowers repo.

| Source Skill | Iron Law / Discipline | Embedded In |
|---|---|---|
| `test-driven-development` | NO PRODUCTION CODE WITHOUT FAILING TEST FIRST | `implement`, `arc-implementer` |
| `systematic-debugging` | NO FIXES WITHOUT ROOT CAUSE INVESTIGATION FIRST | `debug` |
| `verification-before-completion` | NO COMPLETION CLAIMS WITHOUT FRESH VERIFICATION EVIDENCE | `verify` |
| `brainstorming` | No code before design approval (hard gate) | `brainstorm` |
| `writing-plans` | Each step is ONE action, assume zero codebase context | `plan` |
| `requesting/receiving-code-review` | Technical evaluation, not performative agreement | `review` |

### What's NOT Extracted

| Superpowers concept | Why excluded |
|---|---|
| Worktree management | Orthogonal — use superpowers' worktree skill if wanted |
| Skill-writing methodology | Meta-concern, not relevant to arc users |
| Parallel agent dispatch | Arc's `arc-issue-tracker` agent handles bulk ops |
| `using-superpowers` meta-skill | Arc hooks + `arc prime` handle context injection |
| Two-stage review (spec + quality) | Simplified to single review dispatch |

## Composition With Agentic Teams

### What We Build Now (This Design)

- 7 skills: `brainstorm`, `plan`, `implement`, `debug`, `verify`, `review`, `finish`
- 2 agents: `arc-implementer`, `arc-reviewer`

### Already Designed (Agentic Teams Doc, 2026-02-28)

- `arc-team-deploy` skill
- `arc team context` CLI command
- `arc prime` role-aware output (team lead / teammate modes)
- Web Team View

### Connection Point

`plan` creates arc issues with optional `teammate:*` labels → `arc team-deploy` reads those issues. Same data, different execution model.

## Architecture

```
claude-plugin/
├── plugin.json                   # existing — no changes
├── commands/                     # existing 17 commands — unchanged
├── agents/
│   ├── arc-issue-tracker.md      # existing — unchanged
│   ├── arc-implementer.md        # NEW
│   └── arc-reviewer.md           # NEW
└── skills/
    ├── arc/SKILL.md              # existing — update to index new skills
    ├── brainstorm/SKILL.md       # NEW
    ├── plan/SKILL.md             # NEW
    ├── implement/SKILL.md        # NEW
    ├── debug/SKILL.md            # NEW
    ├── verify/SKILL.md           # NEW
    ├── review/SKILL.md           # NEW
    ├── finish/SKILL.md           # NEW
    └── arc-team-deploy/SKILL.md  # FUTURE — from agentic teams design
```

## Token Budget

| Skill | Estimated Size |
|-------|---------------|
| `brainstorm` | ~500 words |
| `plan` | ~600 words |
| `implement` | ~600 words |
| `debug` | ~550 words |
| `verify` | ~350 words |
| `review` | ~400 words |
| `finish` | ~500 words |
| `arc-implementer` agent | ~400 words |
| `arc-reviewer` agent | ~300 words |
| **Total** | **~4,200 words** |

Compare: superpowers' 14 skills loaded individually, each 300-1500 words, plus chain-loading between skills. The arc-native set is self-contained and leaner.

## Verification

```bash
# Skills are markdown — no compilation needed
# Verify by invoking each skill and confirming it:
# 1. Uses arc CLI commands correctly
# 2. Uses TodoWrite for in-session steps (not arc)
# 3. Follows the iron laws / hard gates
# 4. Dispatches subagents for implementation (doesn't implement directly)
# 5. Produces arc artifacts compatible with arc team-deploy
```
