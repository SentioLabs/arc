# Arc Issue Tracker

Track complex, multi-session work with a central issue tracking system. Use this when work spans multiple sessions, has complex dependencies, or requires persistent context.

## When to Use Arc vs TodoWrite

| Use Arc | Use TodoWrite |
|---------|---------------|
| Multi-session work | Single-session tasks |
| Complex dependencies | Linear task lists |
| Discovered work patterns | Simple checklists |
| Work needing audit trail | Quick, disposable lists |
| Strategic planning | Tactical execution |

**Rule of thumb**: When in doubt, prefer arc—persistence you don't need beats lost context.

## Prerequisites

Before using, ensure:
1. arc server is running (`arc-server`)
2. Workspace is initialized (`arc init`)
3. You have a workspace selected (`arc workspace use <name>`)

Check setup: `arc onboard`

## CLI Reference

### Finding Work
```bash
arc ready              # Show issues ready to work (no blockers)
arc list               # List all issues
arc list --status=open # Filter by status
arc show <id>          # Detailed issue view with dependencies
```

### Creating & Updating
```bash
arc create "title" --type=task --priority=2   # New issue
arc update <id> --status=in_progress          # Claim work
arc update <id> --assignee=<name>             # Assign
arc close <id>                                # Mark complete
arc close <id1> <id2> ...                     # Close multiple
```

### Dependencies
```bash
arc dep add <issue> <depends-on>    # Add dependency
arc blocked                          # Show blocked issues
```

### Project Health
```bash
arc stats      # Statistics
arc onboard    # Full orientation
```

## Priority Levels

| Level | Meaning |
|-------|---------|
| P0 | Critical - drop everything |
| P1 | High - urgent |
| P2 | Medium - normal (default) |
| P3 | Low - nice to have |
| P4 | Backlog - someday |

## Issue Types

- **task**: General work item
- **bug**: Something broken
- **feature**: New functionality
- **epic**: Large feature (multiple issues)
- **chore**: Maintenance work

## Session Protocol

### Starting a Session
```bash
arc onboard    # Get context
arc ready      # Find work
```

### Ending a Session

**CRITICAL**: Before saying "done", complete this checklist:

```
[ ] 1. Create issues for remaining work
[ ] 2. Close completed issues
[ ] 3. git add && git commit
[ ] 4. git push
```

Work is NOT done until `git push` succeeds.

## Common Workflows

### Pick Up Work
```bash
arc ready                              # Find available
arc show <id>                          # Review details
arc update <id> --status in_progress   # Claim it
```

### Complete Work
```bash
arc close <id>                    # Close issue
git add . && git commit -m "..."  # Commit
git push                          # Push
```

### Discover Work During Session
```bash
arc create "Found: need to refactor X" --type task
arc dep add <new-issue> <current-issue>  # Link if related
```

## Tips

1. **Run `arc onboard` at session start** - Gets you oriented
2. **Use `arc ready` frequently** - Shows unblocked work
3. **Close issues immediately** - Don't batch
4. **Create issues for discovered work** - Don't lose context
5. **Always push before ending** - Work isn't done until pushed
