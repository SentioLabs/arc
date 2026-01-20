---
description: Close a completed issue
argument-hint: [issue-id] [--reason REASON]
---

Close an arc issue that's been completed.

If arguments are provided:
- $1: Issue ID
- --reason: Completion reason (optional)

If the issue ID is missing, ask for it. Optionally ask for a reason describing what was done.

**Close an issue:**
```bash
arc close <id>
arc close <id> --reason "Implemented in commit abc123"
```

**Close multiple issues:**
```bash
arc close <id1> <id2> <id3>
```

**Closing epics:**
When closing an epic, consider whether all child issues are complete. You can close an epic even if children remain open, but it's better practice to:
1. Check child issues: `arc show <epic-id>`
2. Close remaining children or move them to a new epic
3. Then close the epic

After closing, suggest checking for:
- Dependent issues that might now be unblocked (`arc ready`)
- New work discovered during this task (`arc create` with dependency)
