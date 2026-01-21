---
description: Update an issue's status, priority, or other fields
argument-hint: <issue-id> [--status STATUS] [--priority N]
---

Update an arc issue's fields.

Available updates:
- `--status open|in_progress|blocked|deferred|closed`
- `--priority 0-4`
- `--title "new title"`
- `--description "text"` (use for resumability notes)
- `--assignee NAME`
- `--type bug|feature|task|epic|chore`

Examples:
```bash
arc update <id> --status in_progress   # Start working
arc update <id> --priority 1           # Raise priority
arc update <id> --assignee alice       # Assign to alice
```

If the issue ID is missing, ask for it or suggest `arc list` to find issues.
