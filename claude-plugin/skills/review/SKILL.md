---
name: review
description: Use after implementing a task to get code review. Dispatches the arc-reviewer agent with git diff and task spec, then triages feedback by severity. Works in both single-agent and team contexts.
---

# Review — Code Review Dispatch

Dispatch the `arc-reviewer` subagent to review implementation work, then triage findings.

## Workflow

Create a TodoWrite checklist with these steps:

### 1. Get Git SHAs

Determine the commit range for this task's changes:

```bash
# Number of commits for this task
BASE_SHA=$(git rev-parse HEAD~<n>)
HEAD_SHA=$(git rev-parse HEAD)
```

### 2. Dispatch Reviewer

Use the Agent tool to spawn an `arc-reviewer` subagent with this prompt:

```
Review these changes against the task spec and project conventions.

## Task Spec
<paste output of: arc show <task-id> -w <workspace>>

## Changes
<paste output of: git diff <BASE_SHA>..<HEAD_SHA>>

Report findings as: Critical (must fix), Important (should fix), Minor (note for later).
```

### 3. Triage Feedback

When the reviewer reports back:

| Severity | Action |
|----------|--------|
| **Critical** | Fix immediately — re-dispatch `arc-implementer` with the specific fix. Then re-review. |
| **Important** | Fix before moving to next task — re-dispatch `arc-implementer`. Then re-review. |
| **Minor** | Note in arc issue comment for later. Proceed. |

### 4. Handle Fixes

If fixes are needed:
1. Re-dispatch `arc-implementer` with the specific findings to address
2. After the implementer reports back, re-review (go to step 1 with updated SHAs)
3. Continue until the review is clean (no Critical or Important findings)

### 5. Proceed

- If all tasks are done → invoke `finish`
- If more tasks remain → return to `implement` for the next task

## Response Discipline

When receiving review feedback from the `arc-reviewer`:

- **Evaluate technically.** Don't agree performatively. If a finding is wrong, explain why with evidence.
- **If it's right, fix it.** Don't negotiate or defer valid Critical/Important findings.
- **If it's ambiguous, test it.** Write a test that proves or disproves the concern.
- **No ego.** The reviewer is checking the subagent's work, not yours personally.

## Contexts

This skill works in both execution models:

| Context | How review works |
|---------|-----------------|
| **Single-agent** | Main agent dispatches `arc-reviewer` subagent |
| **Team mode** | Team lead dispatches QA teammate or `arc-reviewer` subagent |

## Rules

- Always review after implementation — don't skip to close
- Re-review after fixes — don't assume fixes are correct
- The reviewer reports; you decide what to do with the findings
- Never make code changes in the review skill — dispatch the implementer for fixes
- Format all arc content (descriptions, plans, comments) per `skills/arc/_formatting.md`
