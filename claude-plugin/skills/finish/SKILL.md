---
name: finish
description: Use at the end of a session to capture remaining work, run quality gates, update arc issues, and commit/push all changes. Replaces both "Landing the Plane" and "finishing-a-development-branch" — one unified session completion protocol.
---

# Finish — Unified Session Completion

Complete the session: capture remaining work, pass quality gates, update arc, commit, push. One protocol for all contexts.

## Iron Law

**Work is NOT done until `git push` succeeds. No exceptions.**

Uncommitted code doesn't exist. Unpushed commits are local fiction. The remote is the source of truth.

## Protocol

Create a TodoWrite checklist with all steps and work through them:

### Phase 1: Capture Remaining Work

1. Review what was planned vs what was completed
2. For any unfinished work or newly discovered tasks:
   ```bash
   arc create "Remaining: <description>" --type=task -w <workspace>
   ```
3. Add context notes to new issues so the next session can pick up:
   ```bash
   arc update <id> --description "CONTEXT: <what was done, what remains, any gotchas>" -w <workspace>
   ```

### Phase 2: Quality Gates

*Skip this phase if no code was changed in this session.*

4. Run project test suite:
   ```bash
   make test    # or: go test ./..., npm test, etc.
   ```
5. Run linter/formatter if configured:
   ```bash
   make lint    # or: golangci-lint run, eslint, etc.
   ```
6. Run build if applicable:
   ```bash
   make build
   ```
7. **Hard gate**: If tests fail, fix them. Do NOT skip to commit. Invoke `debug` if needed.

### Phase 2.5: Human Code Review

*Skip this phase if no code was changed, or if the workspace has no `path` with a `.git` directory.*

8. Determine the server URL. Check if the arc server is running:
   ```bash
   curl -s http://localhost:7432/health | jq -r '.webui_url'
   ```
   If the server is not running or has no web UI, skip this phase.

9. Ask the user how they want to proceed:
   - **Review in browser** — open the web-based diff viewer
   - **Approve without review** — skip directly to Phase 3

10. If the user chose "Review in browser":

    a. Create a review session:
    ```bash
    curl -s -X POST http://localhost:7432/api/v1/workspaces/<ws>/review \
      -H 'Content-Type: application/json' \
      -d '{"base": "origin/main", "head": "HEAD"}' | jq
    ```
    b. Print the review URL for the user:
    ```text
    Review your changes at: http://localhost:7432/<workspaceId>/review?base=origin/main
    ```
    c. Poll the review status every 3 seconds until it changes from "pending":
    ```bash
    curl -s http://localhost:7432/api/v1/workspaces/<ws>/review/<reviewId>/status | jq -r '.status'
    ```
    d. When status changes:
    - `approved` — Print "Review approved" and continue to Phase 3
    - `changes_requested` — Print the review feedback (comment + file comments), then **STOP the finish flow entirely**. The user or AI addresses the feedback, then re-runs `/finish`.

### Phase 3: Update Arc Issues

11. Close completed issues:
    ```bash
    arc close <id> -r "Done: <summary of what was completed>" -w <workspace>
    ```
12. Update in-progress issues with progress notes:
    ```bash
    arc update <id> --description "PROGRESS: <what's done>. NEXT: <what remains>" -w <workspace>
    ```
13. Verify issue states match reality — don't leave stale statuses

### Phase 4: Commit and Push

14. Stage changed files (specific files, not `git add -A`):
    ```bash
    git add <file1> <file2> ...
    ```
15. Commit with conventional commit message:
    ```bash
    git commit -m "feat(scope): summary of changes"
    ```
16. Push:
    ```bash
    git push
    ```
17. Verify push succeeded:
    ```bash
    git status    # Must show "up to date with origin"
    ```
18. If push fails → resolve the issue → retry → succeed. Do not leave unpushed commits.
19. Clean up worktrees:
    ```bash
    git worktree list
    ```
    If only the main working tree is listed, skip ahead. Otherwise, for each extra worktree:

    **a. Check for uncommitted work:**
    ```bash
    git -C <worktree-path> status
    git -C <worktree-path> stash list
    ```
    If there are uncommitted changes or stashes → do NOT remove. Create an arc issue to track the unmerged work:
    ```bash
    arc create "Recover unmerged worktree work: <branch>" --type=task -w <workspace>
    ```

    **b. Check if the branch was merged:**
    ```bash
    git branch --merged | grep <worktree-branch>
    ```
    If merged (or if the worktree is clean with no unique commits), safe to remove:
    ```bash
    git worktree remove <worktree-path>
    git branch -d <worktree-branch>    # Delete the merged branch
    ```

    **c. If the branch has unmerged commits but no uncommitted changes:**
    Check whether the commits exist on a remote:
    ```bash
    git log origin/<worktree-branch> 2>/dev/null
    ```
    If pushed → safe to remove locally. If not pushed → do NOT remove; create an arc issue.

    **d. Prune stale worktree references:**
    ```bash
    git worktree prune
    ```

### Phase 5: Verify and Hand Off

20. Confirm the commit:
    ```bash
    git log -1    # Verify latest commit is visible
    ```
21. Output context for next session:
    ```bash
    arc prime -w <workspace>
    ```

## Context-Aware Behavior

| Session Type | Behavior |
|-------------|----------|
| **Single-agent** | Full protocol above |
| **Team lead** | Verify teammate work → close arc issues → team cleanup → commit → push |
| **Teammate** | Commit → push (team lead handles arc close and coordination) |

## What's NOT in This Protocol

- `git stash clear`, `git remote prune origin` — housekeeping, not gates
- Worktree directory `.gitignore` verification — assumed to be configured at project setup
- Merge/PR/keep/discard choice — arc workflow always commits and pushes
- Performative session summaries — `arc prime` handles handoff context

## Rules

- Never skip Phase 2 (quality gates) when code has changed
- Never commit with `git add -A` — stage specific files
- Never leave unpushed commits
- Never close arc issues without completing the work
- Always run `arc prime` at the end for next-session context
- Format all arc content (descriptions, plans, comments) per `skills/arc/_formatting.md`
- When the user requests changes via the review UI, stop the finish flow — do not continue to commit/push
- The review phase is skippable (approve without review) to avoid blocking trivial sessions
