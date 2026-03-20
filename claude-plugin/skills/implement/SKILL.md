---
name: implement
description: You MUST use this skill to execute implementation tasks from an arc plan — especially when the user says "implement this", "build this", "execute the plan", "start coding", or wants to dispatch subagents for TDD execution of arc issues. The main agent orchestrates; it never writes implementation code directly. Always prefer this over generic implementation when the project uses arc issue tracking.
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

**Task tracking**: At the start of implementation, create a task list using `TaskCreate` with one entry per arc issue to implement. This provides a visible progress tracker in the CLI. Update each task as you work:
- `in_progress` when dispatching the subagent
- `completed` when the task is closed in arc

```bash
# Get the list of tasks to implement
arc list --parent=<epic-id> --status=open --json
```

Create a `TaskCreate` entry for each, then work through this loop:

### 1. Find Next Task

```bash
arc ready
# or for a specific epic:
arc list --parent=<epic-id> --status=open
```

### 2. Claim Task

```bash
arc update <task-id> --status in_progress
```

### 3. Dispatch Agent

Record the current HEAD before dispatching — needed for review if escalated:

```bash
PRE_TASK_SHA=$(git rev-parse HEAD)
```

Check whether the task has a `docs-only` label:

```bash
arc show <task-id> --json | jq -e '.labels[] | select(. == "docs-only")' > /dev/null 2>&1
```

**If `docs-only`** (exit code 0) — spawn an `arc-doc-writer` subagent:

```
Write/update the documentation described in this task.

## Task
<paste output of: arc show <task-id>>

Verify formatting quality and commit your work.
```

**Otherwise** — spawn an `arc-implementer` subagent:

```
Implement this task following TDD (RED → GREEN → REFACTOR → GATE).

## Task
<paste output of: arc show <task-id>>

## Project Test Command
<project's test command, e.g., make test, go test ./...>

Commit your work when all gate checks pass.
```

### 4. Evaluate Result

When the subagent reports back, check the **Result** and **Gate Results** in its report:

**If `PASS`** (all gate checks passed):
- Run the project test command fresh yourself to confirm — do NOT trust the subagent's report alone
- If tests pass → proceed to step 5 (Dispatch Review)

**If `PARTIAL`** (gate identified unresolved issues):
- Read the `Gate: Unresolved` section carefully
- Decide: is this a re-dispatch or a debug situation?
- Handle issues before proceeding (see below)

**If the subagent did not include gate results** (it skipped the gate):
- Treat this as a failed result — re-dispatch with explicit reminder to complete all gate checks

**Handling issues from PARTIAL results**:

- **Subagent reports `PARTIAL` with clear gaps** — re-dispatch `arc-implementer` with the specific gaps listed in `Gate: Unresolved`, plus the original task description
- **Subagent reports test failures it can't resolve** — invoke the `debug` skill
- **3+ implementation attempts fail on same issue** — invoke the `debug` skill
- **Approach was wrong** — re-dispatch the appropriate agent with corrected guidance

When re-dispatching, include the previous gate feedback so the implementer knows exactly what to fix:

```
Continue implementing this task. A previous attempt was made but the gate check identified issues.

## Task
<paste output of: arc show <task-id>>

## Previous Gate Feedback
<paste the Gate: Unresolved section from the previous report>

## Project Test Command
<project's test command>

Fix the identified issues, re-run all gate checks, and commit when complete.
```

### 5. Dispatch Review

After confirming tests pass, invoke the `review` skill to check both code quality and plan adherence:

```bash
# Get the design context from the parent epic
PARENT=$(arc show <task-id> --json | jq -r '.parent_id // empty')
```

The review skill handles retrieving the full design excerpt and dispatching the reviewer. Pass the task ID and the PRE_TASK_SHA recorded in step 3.

> **Note**: For `docs-only` tasks, review remains optional. Skip this step unless the documentation changes are substantial or affect developer-facing API docs.

### 6. Handle Review Findings

Process the review skill's triage results:

| Finding | Action |
|---------|--------|
| **Critical/Important** | Re-dispatch `arc-implementer` with fixes. Re-review after. |
| **Minor** | Note in arc comment. Proceed. |
| **Deviation (fix)** | Re-dispatch `arc-implementer` to match the design. |
| **Deviation (accept)** | Log as arc comment: "Accepted deviation: \<description\>. Rationale: \<why\>." Proceed. |

For accepted deviations, the orchestrator decides — not the reviewer. If unsure whether a deviation is an improvement, default to fixing it to match the plan.

### 7. Close Task

```bash
arc close <task-id> -r "Implemented: <summary>"
```

### 8. Integration Checkpoint

After closing 2-3 related tasks, or before switching to a new epic phase, run the full integration test suite:

```bash
make test-integration
```

This catches cross-task regressions that individual implementer gate checks won't — each implementer only validates its own task's scope. Do not wait until all tasks are complete to discover integration failures.

If integration tests fail:
- Identify which task's changes caused the failure
- Re-dispatch `arc-implementer` with the failing test details and the relevant task context
- If the failure spans multiple tasks, invoke the `debug` skill

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
arc show <task-id>
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
arc ready                           # Find next task
arc update <id> --status in_progress  # Claim task
arc show <id>                        # Get task description for subagent
arc close <id> -r "reason"            # Close completed task
```

## Rules

- Never write implementation code as the main agent — always dispatch
- Never close a task without confirming tests pass yourself (fresh run)
- Never close a task if the implementer reported `PARTIAL` without re-dispatching
- If in doubt about the result, re-dispatch rather than fixing manually
- Never dispatch parallel agents without committing and pushing all sequential work first
- Never dispatch parallel agents on tasks that share files
- Never proceed after parallel merge without verifying commit history against the recorded HEAD anchor
- Never mix sequential and parallel dispatch in the same batch — finish one mode before switching to the other
- Format all arc content (descriptions, plans, comments) per `skills/arc/_formatting.md`
