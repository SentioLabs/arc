---
name: plan
description: You MUST use this skill to break a design or feature into implementation tasks — especially after brainstorming, when the user says "plan this", "break this down", "create tasks", or wants to turn a design into actionable arc issues with exact file paths. Creates self-contained arc issues that subagents can implement with zero prior context. Always prefer this over generic planning when the project uses arc issue tracking.
---

# Plan — Implementation Task Breakdown

Break an approved design into bite-sized, self-contained tasks with exact file paths and steps.

## Plan Commands

Plans are ephemeral review artifacts backed by filesystem markdown files:
- `arc plan create --file <path>` — Register a plan for review, returns plan ID
- `arc plan show <plan-id>` — Show plan content, status, and comments
- `arc plan approve <plan-id>` — Approve the plan
- `arc plan reject <plan-id>` — Reject the plan
- `arc plan comments <plan-id>` — List review comments

## Granularity Rule

Each task step is **ONE action, 2-5 minutes**. Assume the implementer has **zero codebase context** and fresh context without codebase familiarity. If a step says "add validation" without showing the code, it's too vague.

## Workflow

Create a TodoWrite checklist with these steps and work through them:

### 1. Read the Design

```bash
arc plan show <plan-id>
```

Load the approved design from brainstorm. The plan ID is provided by the brainstorm skill after registration. Understand the full scope before breaking it down.

### 2. Identify Shared Contracts (Foundation Task)

Check the design for **shared contracts** — types, interfaces, config keys, constants, or function signatures referenced by multiple tasks. If the brainstorm design includes a shared contracts section, use it as input.

If shared contracts exist and parallel execution is likely:

1. Create a **T0: Foundation** task that establishes all shared contracts
2. Mark all parallelizable tasks as **blocked by T0**
3. T0 runs sequentially before any parallel batch begins

This ensures parallel agents inherit shared definitions from HEAD rather than inventing them independently.

**Skip this step** if the work is purely sequential or no shared contracts were identified.

### 3. Identify Tasks

Break the design into self-contained implementation units. Each task should:
- Have a clear, testable outcome
- Be implementable without knowledge of other tasks
- Include exact file paths for all files to create or modify
- Follow a logical dependency order
- **Not overlap in file ownership with other parallelizable tasks**

When identifying tasks, assign **file ownership** — each file should be owned by exactly one task. If two tasks need to modify the same file, either merge them into one task, serialize them with a dependency, or extract the shared file into the foundation task.

### 4. Create Arc Issues via arc-issue-tracker

**Never run `arc create` directly** — always delegate to the `arc-issue-tracker` agent. This keeps bulk CLI output in a disposable subagent context.

Build a task manifest and dispatch it:

```
Use the Agent tool with subagent_type="arc:arc-issue-tracker":

Create the following tasks under epic <epic-id>.
After creation, set dependencies and labels as listed.
Return a summary table mapping task names to arc IDs.

## Tasks

### T1: <title>
Type: task
Parent: <epic-id>
Description:
<full multi-line description>

### T2: <title>
Type: task
Parent: <epic-id>
Description:
<full multi-line description>

## Dependencies
- T2 blocked by T1
- T4 blocked by T3

## Labels
- T3: docs-only

## Required Output
| Task | Arc ID | Title |
|------|--------|-------|
| T1   | ...    | ...   |
```

For each task, check whether **all** files in its `## Files` section are documentation (`.md`, `.txt`, `README`, `CHANGELOG`, or anything under `docs/`). If so, include it in the `## Labels` section with `docs-only`. Doc-only tasks skip TDD — the `implement` skill routes them to `arc-doc-writer` instead of `arc-implementer`.

### 5. Validate Returned Results

Before proceeding, verify the agent's output:

1. **Count check**: The number of returned IDs must match the number of tasks in your manifest
2. **Spot-check**: Run `arc show <id>` on one returned task to confirm it exists and has the correct parent
3. **If mismatch**: Re-dispatch the agent for missing tasks only, or create them manually

### 6. Update Epic with Implementation Breakdown

Using the task IDs from the agent's returned summary table, write the approved design content and task breakdown into the epic's description field:

```bash
arc update <epic-id> --stdin <<'EOF'
<approved design content with task listing>
EOF
```

This stores the implementation breakdown directly on the epic for reference during execution.

### 7. Choose Execution Path

**Use the AskUserQuestion tool** to let the user choose:

```
Question: "How should we execute these tasks?"
Options:
  - "Single-agent + subagents" (invoke /arc:implement now — main agent orchestrates, arc-implementer subagents do TDD per task)
  - "Agentic team" (add teammate labels, invoke arc team-deploy — best for parallel multi-role work)
  - "New session" (start a fresh session and run /arc:implement there — keeps this session free for other work)
```

All three paths lead to `/arc:implement` — the only difference is where and how it runs. After the user chooses, explicitly state the next step:
- Single-agent: invoke the `implement` skill immediately
- Agentic team: add teammate labels, then invoke the `implement` skill
- New session: tell the user to run `/arc:implement <epic-id>` in the new session

If team, add teammate labels when creating the issues — include `teammate:<role>` in the task description for the `arc-issue-tracker` agent to handle during creation, or apply them via the web UI.

Available roles: `teammate:frontend`, `teammate:backend`, `teammate:architect`, `teammate:tests`, `teammate:devops`, or custom.

## Task Description Format

Each task's `--description` must be **self-contained** (~3-5k tokens). The task description IS the implementation context — the implementer loads `arc show <task-id>` and nothing else.

Include in every task description:

```
## Files
- Create: `path/to/new_file.go`
- Modify: `path/to/existing_file.go`
- Test: `path/to/file_test.go`

## Scope Boundary
Do NOT create or modify any files outside the Files section above.
If you need a type, interface, or constant that doesn't exist, do NOT create it —
the foundation task or a prior task is responsible for shared definitions.

## Steps
1. Write failing test for <specific behavior> in `path/to/file_test.go`
2. Run `go test ./path/to/...` — confirm it fails with <expected error>
3. Implement <specific function> in `path/to/new_file.go`:
   - <concrete code guidance, not just "add validation">
4. Run `go test ./path/to/...` — confirm it passes
5. Commit: `feat(module): add <feature>`

## Test Command
go test ./path/to/...

## Expected Outcome
<what should work when this task is done>
```

For `docs-only` tasks, omit `## Test Command` and use `## Verification` instead:

```
## Verification
- All internal links resolve to existing files
- Heading hierarchy has no skipped levels
- Code blocks have language tags
```

## Rules

- Never reference external docs or the full plan in task descriptions — everything needed is in the description
- Design documents live in `docs/plans/` and are registered via `arc plan create --file`
- Task descriptions must include actual code guidance, not vague instructions
- Team preparation (teammate labels) is optional — only if user chooses team execution
- The plan skill creates tasks; it does not implement them
- The plan skill never runs `arc create` directly — always delegate to `arc-issue-tracker`
- Every task must include a `## Scope Boundary` section — no file modifications outside the `## Files` list
- No two parallelizable tasks may own the same file — resolve overlaps via foundation task, merging, or serialization
- Format all arc content (descriptions, plans, comments) per `skills/arc/_formatting.md`
