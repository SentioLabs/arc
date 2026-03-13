# Eval 1 Output: Implement ARC-5.1 (JWT Token Generation)

## Key Finding: The Main Agent Never Writes Implementation Code

The implement skill's core rule is explicit: **"The main agent NEVER writes implementation code. It orchestrates, dispatches, and reviews."** I would NOT write any JWT implementation code directly. Instead, I would dispatch an `arc-implementer` subagent to do the work.

## Step-by-Step Plan

### Step 1: Find the Task

Run `arc list --parent=ARC-5 --status=open` to see all tasks under epic ARC-5, or `arc show ARC-5.1` to inspect the specific task. This confirms the task exists, its description, dependencies, and labels.

### Step 2: Claim the Task

```bash
arc update ARC-5.1 --status in_progress
```

This signals to other agents/sessions that ARC-5.1 is being worked on.

### Step 3: Record Pre-Task SHA and Check Labels

```bash
PRE_TASK_SHA=$(git rev-parse HEAD)
arc show ARC-5.1 --json | jq -e '.labels[] | select(. == "docs-only")' > /dev/null 2>&1
```

If the task has a `docs-only` label, I would dispatch an `arc-doc-writer` subagent instead. For JWT token generation, this is almost certainly NOT docs-only, so I would dispatch an `arc-implementer`.

### Step 4: Dispatch the arc-implementer Subagent

I would use the Agent tool (Task tool) to spawn a fresh `arc-implementer` subagent with:

```
Implement this task following TDD (RED -> GREEN -> REFACTOR -> GATE).

## Task
<paste full output of: arc show ARC-5.1>

## Project Test Command
make test

Commit your work when all gate checks pass.
```

The subagent gets a clean context window with just the task description. It follows TDD: write a failing test first, make it pass, refactor, then run gate checks. I do NOT write any Go code, test files, or JWT logic myself.

### Step 5: Evaluate the Subagent's Result

When the subagent reports back, I check its **Result** and **Gate Results**:

- **If PASS**: I run `make test` myself to independently verify. I do NOT trust the subagent's report alone.
- **If PARTIAL**: I read the `Gate: Unresolved` section and re-dispatch with the specific gaps.
- **If no gate results reported**: I treat it as failed and re-dispatch with an explicit reminder to complete all gate checks.

### Step 6: Handle Issues (if any)

- If the subagent reports PARTIAL with clear gaps, re-dispatch with the previous gate feedback included.
- If 3+ attempts fail on the same issue, invoke the `debug` skill instead.
- I never "just quickly fix" something myself -- I always re-dispatch.

### Step 7: Close the Task

After confirming tests pass with my own fresh `make test` run:

```bash
arc close ARC-5.1 -r "Implemented: JWT token generation"
```

### Step 8: Integration Checkpoint

Since this is the first task in the epic, I would note that after closing 2-3 related tasks (e.g., after ARC-5.1 and ARC-5.2), I would run `make test-integration` to catch cross-task regressions.

### Step 9: Repeat

Return to Step 1 for the next task in ARC-5 (e.g., ARC-5.2).

## Summary

The implement skill enforces a strict separation: the main agent is an **orchestrator**, not an implementer. For ARC-5.1, I would:

1. Claim the task via `arc update`
2. Dispatch an `arc-implementer` subagent with the task description and test command
3. Verify the result independently by running `make test` myself
4. Close the task only after independent verification passes
5. Move to the next task in the epic

I would **never** write JWT implementation code, test files, or any source code directly. All implementation work is delegated to subagents.
