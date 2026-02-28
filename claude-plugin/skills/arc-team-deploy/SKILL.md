# Arc Team Deploy

Deploy an agent team from arc's issue graph. Translates `teammate:*` labels, plans, and dependencies into a Claude Code team with tasks and role-filtered context.

## When to Invoke

- User says "deploy team", "create agent team from arc", "spawn teammates from arc"
- User runs `/arc team-deploy`
- User wants to parallelize work on an arc epic across multiple agents

## Prerequisites

- An arc workspace is active (`.arc.json` exists)
- An epic exists with child issues labeled `teammate:<role>` (e.g., `teammate:frontend`, `teammate:backend`)
- The arc server is running (`arc server status`)

## Workflow

### Step 1: Gather Team Context

Run `arc team context <epic-id> --json` to get the issue graph grouped by role.

```bash
arc team context <epic-id> --json
```

If the user hasn't specified an epic, help them find one:

```bash
arc list --type=epic --status=open
```

### Step 2: Present Team Composition

Parse the JSON output and present a summary for approval:

```
Team composition for "<epic-title>":

  frontend (2 issues): Login form, Signup page
  backend  (3 issues): Auth API, User model, Session middleware
  unassigned (1 issue): Write tests

Proceed with team deployment? [Y/n]
```

If there are unassigned issues, suggest either assigning them to a role or leaving them for the lead to handle.

### Step 3: Create Team and Tasks

After approval:

1. **Create the team** via `TeamCreate`:
   ```
   team_name: "<epic-title-slug>"
   description: "Working on <epic-title>"
   ```

2. **Create tasks** via `TaskCreate` for each arc issue:
   - `subject`: The arc issue title
   - `description`: Include the arc issue ID, plan (if any), dependencies, and priority
   - `activeForm`: Present continuous of the subject (e.g., "Implementing login form")

3. **Set task dependencies** via `TaskUpdate` with `addBlockedBy`:
   - Map arc dependency IDs to the corresponding task IDs
   - Only map dependencies between issues within the same team deployment

### Step 4: Spawn Teammates

For each role, spawn a teammate via the `Agent` tool:

```
subagent_type: "general-purpose"
team_name: "<team-name>"
name: "<role>"  (e.g., "frontend", "backend")
```

**Prompt template** for each teammate:

```
You are the <role> teammate working on "<epic-title>".

Your arc role label is teammate:<role>. Focus on your assigned tasks.

Environment: ARC_TEAMMATE_ROLE=<role>

Workflow:
1. Check TaskList for your assigned tasks
2. Work on tasks in ID order (lowest first)
3. For each task: mark in_progress, implement, run tests, mark completed
4. After completing a task, check TaskList for the next available one
5. Send a message to the team lead when all your tasks are done

Arc context for your issues:
<paste role-specific issues from the team context JSON>
```

### Step 5: Assign Tasks

Use `TaskUpdate` with `owner` to assign each task to the corresponding teammate role name.

### Step 6: Monitor and Sync

As team lead, follow the sync protocol:

1. **Monitor progress** via `TaskList` — teammates send messages on completion
2. **Verify work** before closing arc issues:
   ```bash
   arc show <issue-id>   # Review the issue
   # Check the code changes made by the teammate
   ```
3. **Close verified issues**:
   ```bash
   arc close <issue-id> --reason "completed by <role>"
   ```
4. **Check for newly unblocked work**:
   ```bash
   arc ready
   ```
5. **Shutdown teammates** when all work is complete via `SendMessage` with `type: "shutdown_request"`

## Error Handling

- If `arc team context` returns empty roles, the epic may not have `teammate:*` labels on children. Suggest labeling first.
- If a teammate reports a blocker, update the arc issue status: `arc update <id> --status=blocked`
- If a task fails, investigate before reassigning — the arc issue may need plan revision.

## Example Session

```
User: Deploy a team for epic PROJ-5

1. Run: arc team context PROJ-5 --json
2. Parse: 2 roles (frontend: 2 issues, backend: 3 issues)
3. Present composition → user approves
4. TeamCreate: "auth-system"
5. TaskCreate: 5 tasks (mapped from arc issues)
6. TaskUpdate: set dependencies between tasks
7. Agent spawn: "frontend" teammate (2 assigned tasks)
8. Agent spawn: "backend" teammate (3 assigned tasks)
9. Monitor via TaskList, verify completions
10. arc close verified issues
11. Shutdown teammates when done
```