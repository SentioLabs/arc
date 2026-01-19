# Arc Issue Tracker

Track complex, multi-session work with a central issue tracking system.

## When to Use Arc vs TodoWrite

| Use Arc | Use TodoWrite |
|---------|---------------|
| Multi-session work | Single-session tasks |
| Complex dependencies | Linear task lists |
| Discovered work patterns | Simple checklists |
| Work needing audit trail | Quick, disposable lists |

**Rule of thumb**: When in doubt, prefer arc—persistence you don't need beats lost context.

## CLI Reference

Run `arc prime` for full workflow context, or `arc <command> --help` for specific commands.

**Essential commands:**
- `arc ready` - Find unblocked work
- `arc create` - Create issues
- `arc update` - Update status/fields
- `arc close` - Complete work
- `arc show` - View details
- `arc dep` - Manage dependencies

## Agent Mode

For bulk operations (creating epics with tasks, batch updates), use the **arc-issue-tracker** agent via the Task tool. This runs arc commands without consuming main conversation context.

## Session Protocol

**Before ending any session:**
```
git status → git add → git commit → git push
```

Work is NOT done until `git push` succeeds.
