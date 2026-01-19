# Beads-Central Issue Tracker

Track complex, multi-session work with a central issue tracking system. Use this when work spans multiple sessions, has complex dependencies, or requires persistent context.

## When to Use Beads-Central vs TodoWrite

| Use Beads-Central | Use TodoWrite |
|-------------------|---------------|
| Multi-session work | Single-session tasks |
| Complex dependencies | Linear task lists |
| Discovered work patterns | Simple checklists |
| Work needing audit trail | Quick, disposable lists |
| Strategic planning | Tactical execution |

**Rule of thumb**: When in doubt, prefer bd—persistence you don't need beats lost context.

## Prerequisites

Before using, ensure:
1. beads-central server is running (`beads-server`)
2. Workspace is initialized (`bd init`)
3. You have a workspace selected (`bd workspace use <name>`)

Check setup: `bd onboard`

## CLI Reference

### Finding Work
```bash
bd ready              # Show issues ready to work (no blockers)
bd list               # List all issues
bd list --status=open # Filter by status
bd show <id>          # Detailed issue view with dependencies
```

### Creating & Updating
```bash
bd create "title" --type=task --priority=2   # New issue
bd update <id> --status=in_progress          # Claim work
bd update <id> --assignee=<name>             # Assign
bd close <id>                                # Mark complete
bd close <id1> <id2> ...                     # Close multiple
```

### Dependencies
```bash
bd dep add <issue> <depends-on>    # Add dependency
bd blocked                          # Show blocked issues
```

### Project Health
```bash
bd stats      # Statistics
bd onboard    # Full orientation
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
bd onboard    # Get context
bd ready      # Find work
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
bd ready                              # Find available
bd show <id>                          # Review details
bd update <id> --status in_progress   # Claim it
```

### Complete Work
```bash
bd close <id>                    # Close issue
git add . && git commit -m "..."  # Commit
git push                          # Push
```

### Discover Work During Session
```bash
bd create "Found: need to refactor X" --type task
bd dep add <new-issue> <current-issue>  # Link if related
```

## Tips

1. **Run `bd onboard` at session start** - Gets you oriented
2. **Use `bd ready` frequently** - Shows unblocked work
3. **Close issues immediately** - Don't batch
4. **Create issues for discovered work** - Don't lose context
5. **Always push before ending** - Work isn't done until pushed
