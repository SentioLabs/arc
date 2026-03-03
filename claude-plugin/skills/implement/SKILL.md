---
name: implement
description: Use to execute implementation tasks from a plan. Dispatches fresh subagents per task for context-efficient TDD execution. The main agent orchestrates — it never writes implementation code.
---

# Implement — Subagent-Driven TDD Execution

Orchestrate task implementation by dispatching fresh `arc-implementer` subagents per task. Each subagent gets a clean context window with just the task description.

## Core Rule

**The main agent NEVER writes implementation code.** It orchestrates, dispatches, and reviews. If you're tempted to "just quickly fix this" — dispatch a subagent instead.

## Dispatch Modes

### Sequential (default)

Tasks are dispatched one at a time through the orchestration loop below. Use this for:
- Most workflows — it's the safe default
- Tasks with any file overlap
- Tasks with dependency ordering (`blocks`/`blockedBy`)
- When you're unsure whether tasks are independent

### Parallel

Multiple tasks dispatched simultaneously using `isolation: "worktree"`. Use this **only** when ALL of these are true:
- 3+ independent tasks remain
- No shared files between any tasks in the batch
- No `blocks`/`blockedBy` dependencies between tasks in the batch
- Each task's scope is clearly defined with no ambiguity

**When NOT to use parallel**: overlapping files, task dependencies, uncertainty about scope, fewer than 3 tasks. Default to sequential — the cost of serial execution is time; the cost of a bad parallel merge is data loss.

## Orchestration Loop

By default, use sequential dispatch. For independent tasks, see [Parallel Dispatch Protocol](#parallel-dispatch-protocol) below.

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

Record the current HEAD before dispatching — the review skill needs this to determine the commit range:

```bash
PRE_TASK_SHA=$(git rev-parse HEAD)
```

Check whether the task has a `docs-only` label:

```bash
arc show <task-id> -w <workspace> --json | jq -e '.labels[] | select(. == "docs-only")' > /dev/null 2>&1
```

**If `docs-only`** (exit code 0) — spawn an `arc-doc-writer` subagent:

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

When the subagent reports back, invoke the `verify` skill to confirm tests pass and the task spec is met. Specifically check:
- Did all tests pass? (run the proof command fresh — don't trust the subagent's report alone)
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

## Parallel Dispatch Protocol

When you have identified a batch of truly independent tasks (see [Dispatch Modes](#dispatch-modes)), switch from the sequential loop to this protocol:

### P1. Commit Checkpoint

Before switching to parallel, ensure all sequential work is committed and pushed:

```bash
git status          # Must be clean — no unstaged or uncommitted changes
git log -3          # Verify recent sequential commits are present
git push            # Establish a recovery point on the remote
```

**Hard gate**: Do NOT proceed if `git status` shows uncommitted changes.

### P2. Record HEAD Anchor

```bash
PARALLEL_BASE=$(git rev-parse HEAD)
echo "Parallel base: $PARALLEL_BASE"
```

This is the baseline all worktrees will branch from. Record it — you'll need it for verification after merge.

### P3. Verify Independence

For each task in the planned parallel batch:

```bash
arc show <task-id> -w <workspace>
```

Confirm:
- No `blocks`/`blockedBy` relationships between tasks in this batch
- No overlapping file paths in task descriptions
- Each task has a clearly scoped, non-ambiguous specification

If any task fails these checks, remove it from the parallel batch and handle it sequentially after.

### P4. Dispatch in Single Turn

All parallel Agent tool calls with `isolation: "worktree"` **must happen in the same orchestrator message**. This ensures they all branch from the same HEAD.

```
# In a single response, dispatch all parallel tasks:
Agent(subagent_type="arc-implementer", isolation="worktree", prompt="Task 1...")
Agent(subagent_type="arc-implementer", isolation="worktree", prompt="Task 2...")
Agent(subagent_type="arc-implementer", isolation="worktree", prompt="Task 3...")
```

**Never** dispatch worktree agents across multiple turns — HEAD may move between turns, causing stale branches.

### P5. Merge-Back Verification

After all parallel agents report back, verify the merge did not lose work:

```bash
# 1. Check HEAD against the recorded anchor
git log --oneline $PARALLEL_BASE..HEAD    # Should show ONLY the parallel agents' commits

# 2. Verify sequential commits are still in history
git log --oneline HEAD | head -20         # All prior sequential commits must be present

# 3. Run full test suite
make test    # or project-specific test command
```

**If sequential commits are missing** → STOP. Do not continue. Recover from reflog:

```bash
git reflog                                # Find the pre-merge state
git log --oneline <reflog-ref>            # Verify it has the missing commits
# Cherry-pick or reset as appropriate — ask user if unsure
```

### P6. Resume Sequential

After successful verification, return to the normal orchestration loop (step 1) for any remaining tasks.

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
- Never dispatch parallel agents without committing and pushing all sequential work first
- Never dispatch parallel agents on tasks that share files
- Never proceed after parallel merge without verifying commit history against the recorded HEAD anchor
- Never mix sequential and parallel dispatch in the same batch — finish one mode before switching to the other
- Format all arc content (descriptions, plans, comments) per `skills/arc/_formatting.md`
