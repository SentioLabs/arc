---
name: implement
description: Use to execute implementation tasks from a plan. Dispatches fresh subagents per task for context-efficient TDD execution. The main agent orchestrates — it never writes implementation code.
---

# Implement — Subagent-Driven TDD Execution

Orchestrate task implementation by dispatching fresh `arc-implementer` subagents per task. Each subagent gets a clean context window with just the task description.

## Core Rule

**The main agent NEVER writes implementation code.** It orchestrates, dispatches, and reviews. If you're tempted to "just quickly fix this" — dispatch a subagent instead.

## Orchestration Loop

Create a TodoWrite checklist and work through this loop for each task:

### 1. Find Next Task

```bash
arc ready -w <workspace>
# or for a specific epic:
arc list --parent=<epic-id> --status=open -w <workspace>
```

### 2. Claim Task

```bash
arc update <task-id> --status in_progress -w <workspace>
```

### 3. Dispatch Agent

Check whether the task has a `docs-only` label:

```bash
arc show <task-id> -w <workspace> | grep -q 'docs-only'
```

**If `docs-only`** — spawn an `arc-doc-writer` subagent:

```
Write/update the documentation described in this task.

## Task
<paste output of: arc show <task-id> -w <workspace>>

Verify formatting quality and commit your work.
```

**Otherwise** — spawn an `arc-implementer` subagent:

```
Implement this task following TDD (RED → GREEN → REFACTOR).

## Task
<paste output of: arc show <task-id> -w <workspace>>

## Project Test Command
<project's test command, e.g., make test, go test ./...>

Commit your work when tests pass.
```

### 4. Review Result

When the subagent reports back, check:
- Did all tests pass?
- Was the approach correct per the task spec?
- Were there any regressions?

### 5. Handle Issues

- **Subagent reports test failures it can't resolve** → invoke the `debug` skill
- **3+ implementation attempts fail on same issue** → invoke the `debug` skill
- **Approach was wrong** → re-dispatch the appropriate agent (`arc-implementer` or `arc-doc-writer`) with corrected guidance

### 6. Review Code

If the result looks clean, invoke the `review` skill to dispatch the `arc-reviewer` subagent. For `docs-only` tasks, code review is optional — skip it unless the documentation changes are substantial or affect developer-facing API docs.

### 7. Process Review Feedback

- **Critical findings** → re-dispatch implementer with the specific fixes
- **Important findings** → re-dispatch implementer before moving to next task
- **Minor findings** → note in an arc comment for later: `arc update <task-id> --description "Minor: ..."  -w <workspace>`

### 8. Close Task

```bash
arc close <task-id> -r "Implemented: <summary>" -w <workspace>
```

### 9. Repeat

Go to step 1 for the next task. Continue until all tasks in the epic are closed.

## When to Invoke Debug

- Subagent reports test failures it can't resolve after reasonable effort
- 3+ implementation attempts fail on the same issue
- A regression appears that isn't explained by the current task's changes

## Arc Commands Used

```bash
arc ready -w <workspace>                           # Find next task
arc update <id> --status in_progress -w <workspace>  # Claim task
arc show <id> -w <workspace>                        # Get task description for subagent
arc close <id> -r "reason" -w <workspace>            # Close completed task
```

## Rules

- Never write implementation code as the main agent — always dispatch
- Never skip the review step after implementation
- Never close a task without checking that tests pass
- If in doubt about the result, re-dispatch rather than fixing manually
- Format all arc content (descriptions, plans, comments) per `skills/arc/_formatting.md`
