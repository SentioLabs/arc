---
name: plan
description: Use after brainstorm to break an approved design into implementation tasks with exact file paths and steps. Creates self-contained arc issues that subagents can implement with zero prior context.
---

# Plan — Implementation Task Breakdown

Break an approved design into bite-sized, self-contained tasks with exact file paths and steps.

## Granularity Rule

Each task step is **ONE action, 2-5 minutes**. Assume the implementer has **zero codebase context** and fresh context without codebase familiarity. If a step says "add validation" without showing the code, it's too vague.

## Workflow

Create a TodoWrite checklist with these steps and work through them:

### 1. Read the Design

```bash
arc plan show <epic-id> -w <workspace>
```

Load the approved design from brainstorm. Understand the full scope before breaking it down.

### 2. Identify Tasks

Break the design into self-contained implementation units. Each task should:
- Have a clear, testable outcome
- Be implementable without knowledge of other tasks
- Include exact file paths for all files to create or modify
- Follow a logical dependency order

### 3. Create Arc Issues via arc-issue-tracker

**Never run `arc create` directly** — always delegate to the `arc-issue-tracker` agent. This keeps bulk CLI output in a disposable subagent context.

Build a task manifest and dispatch it:

```
Use the Agent tool with subagent_type="arc:arc-issue-tracker":

Create the following tasks under epic <epic-id> in workspace <workspace>.
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

### 4. Validate Returned Results

Before proceeding, verify the agent's output:

1. **Count check**: The number of returned IDs must match the number of tasks in your manifest
2. **Spot-check**: Run `arc show <id> -w <workspace>` on one returned task to confirm it exists and has the correct parent
3. **If mismatch**: Re-dispatch the agent for missing tasks only, or create them manually

### 5. Update Epic Plan

Using the task IDs from the agent's returned summary table, add the task breakdown to the epic's plan:
```bash
arc plan set <epic-id> "<updated plan with task listing>" -w <workspace>
```

### 6. Choose Execution Path

Ask the user:
- **Single-agent + subagents**: Invoke the `implement` skill. Main agent orchestrates, `arc-implementer` subagents do TDD per task. Best for sequential tasks.
- **Agentic team**: Add `teammate:*` labels per task, invoke `arc team-deploy`. Best for parallel multi-role work.

If team, prompt for labels:
```bash
arc label add <task-id> teammate:frontend -w <workspace>
arc label add <task-id> teammate:backend -w <workspace>
```

Available roles: `teammate:frontend`, `teammate:backend`, `teammate:architect`, `teammate:tests`, `teammate:devops`, or custom.

## Task Description Format

Each task's `--description` must be **self-contained** (~3-5k tokens). The task description IS the implementation context — the implementer loads `arc show <task-id>` and nothing else.

Include in every task description:

```
## Files
- Create: `path/to/new_file.go`
- Modify: `path/to/existing_file.go`
- Test: `path/to/file_test.go`

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
- Never create `docs/plans/` markdown files — arc plan is the sole artifact
- Task descriptions must include actual code guidance, not vague instructions
- Team preparation (teammate labels) is optional — only if user chooses team execution
- The plan skill creates tasks; it does not implement them
- The plan skill never runs `arc create` directly — always delegate to `arc-issue-tracker`
- Format all arc content (descriptions, plans, comments) per `skills/arc/_formatting.md`
