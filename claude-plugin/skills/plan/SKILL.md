---
name: plan
description: Use after brainstorm to break an approved design into implementation tasks with exact file paths and steps. Creates self-contained arc issues that subagents can implement with zero prior context.
---

# Plan — Implementation Task Breakdown

Break an approved design into bite-sized, self-contained tasks with exact file paths and steps.

## Granularity Rule

Each task step is **ONE action, 2-5 minutes**. Assume the implementer has **zero codebase context** and questionable testing discipline. If a step says "add validation" without showing the code, it's too vague.

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

### 3. Create Arc Issues

For each task:
```bash
arc create "Task title" --type=task --parent=<epic-id> --description="<full task description>" -w <workspace>
```

### 4. Set Dependencies

Where task order matters:
```bash
arc dep add <later-task-id> <earlier-task-id> --type=blocks -w <workspace>
```

### 5. Update Epic Plan

Add the task breakdown to the epic's plan:
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

## Rules

- Never reference external docs or the full plan in task descriptions — everything needed is in the description
- Never create `docs/plans/` markdown files — arc plan is the sole artifact
- Task descriptions must include actual code guidance, not vague instructions
- Team preparation (teammate labels) is optional — only if user chooses team execution
- The plan skill creates tasks; it does not implement them
