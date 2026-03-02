---
name: brainstorm
description: Use when starting a new feature, project, or significant piece of work that needs design exploration before implementation. Guides Socratic discovery of requirements and constraints, then saves the approved design as an arc plan.
---

# Brainstorm — Design Discovery

Explore requirements through Socratic dialogue before any implementation begins.

## Hard Gate

**Do NOT write any implementation code, scaffold any project, or take any implementation action until the design is approved.** Brainstorming produces a design document — not code.

## Workflow

Create a TodoWrite checklist with these steps and work through them:

### 1. Explore Project Context

- Check existing files, docs, recent commits
- Review existing arc issues (`arc list -w <workspace>`)
- Understand what already exists and what constraints are in play

### 2. Ask Clarifying Questions

- Ask questions **one at a time** — don't dump a list
- Prefer multiple-choice questions where reasonable
- Understand: purpose, constraints, success criteria, target users
- Continue until you have enough to propose approaches

### 3. Propose 2-3 Approaches

- Each approach: summary, trade-offs, estimated complexity
- Include a recommendation with reasoning
- Apply YAGNI — remove features from all designs that aren't explicitly required
- Wait for user to choose or refine

### 4. Present Design Section by Section

- Break the design into logical sections (data model, API, UI, etc.)
- Present each section and get user approval before moving to the next
- Iterate on sections as needed based on feedback

### 5. Save to Arc

Detect scale and create appropriate structure:

**Large (multi-phase project):**
```bash
arc create "Project Name" --type=epic -w <workspace>
# Save overall design as plan
arc plan set <meta-epic-id> "<design content>" -w <workspace>
```

If 3+ phases, delegate phase creation to `arc-issue-tracker`:
```
Use the Agent tool with subagent_type="arc:arc-issue-tracker":

Create the following phase epics under meta-epic <meta-epic-id> in workspace <workspace>.
After creation, set dependencies as listed.
Return a summary table mapping phase names to arc IDs.

## Tasks

### P1: Phase 1 - ...
Type: epic
Parent: <meta-epic-id>

### P2: Phase 2 - ...
Type: epic
Parent: <meta-epic-id>

### P3: Phase 3 - ...
Type: epic
Parent: <meta-epic-id>

## Dependencies
- P2 blocked by P1
- P3 blocked by P2

## Required Output
| Phase | Arc ID | Title |
|-------|--------|-------|
| P1    | ...    | ...   |
```

If 1-2 phases, create directly:
```bash
arc create "Phase 1: ..." --type=epic --parent=<meta-epic-id> -w <workspace>
arc create "Phase 2: ..." --type=epic --parent=<meta-epic-id> -w <workspace>
# Set phase dependencies
arc dep add <phase-2-id> <phase-1-id> --type=blocks -w <workspace>
```

**Medium (single feature):**
```bash
arc create "Feature Name" --type=epic -w <workspace>
arc plan set <epic-id> "<design content>" -w <workspace>
```

**Small (single task):**
```bash
arc create "Task Name" --type=task -w <workspace>
# Skip brainstorm/plan — go directly to implement
```

### 6. Transition

- For large/medium work: invoke the `plan` skill to break the design into implementation tasks
- For small work: invoke the `implement` skill directly

## Scale Detection

| Indicator | Scale | Structure |
|-----------|-------|-----------|
| Multiple phases, weeks of work, cross-cutting concerns | Large | Meta epic → phase epics → tasks |
| Single feature, days of work, contained scope | Medium | Epic → tasks |
| One task, hours of work, obvious approach | Small | Single issue |

## Rules

- The ONLY next skill after brainstorm is `plan` (or `implement` for small work)
- Never invoke implementation skills from brainstorm
- Never create `docs/plans/` markdown files — arc plan is the sole artifact
- Arc issues track persistent work; TodoWrite tracks your brainstorming checklist steps
- YAGNI: if the user didn't ask for it, don't design it
- Format all arc content (descriptions, plans, comments) per `skills/arc/_formatting.md`
