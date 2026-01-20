---
description: List issues with optional filters
argument-hint: [--status STATUS] [--type TYPE] [--query SEARCH]
---

List arc issues with optional filtering.

**Common filters:**
- `--status open|in_progress|blocked|deferred|closed`
- `--type bug|feature|task|epic|chore`
- `--assignee NAME`
- `--query "search text"`
- `--limit N`

**Examples:**
```bash
arc list                      # All issues
arc list --status open        # Open issues only
arc list --type bug           # Bugs only
arc list --type epic          # Epics only
arc list --query "auth"       # Search for "auth"
```

**Working with epics:**
```bash
arc list --type epic          # Find all epics
arc show <epic-id>            # View epic and its children
```

Present results in a clear format showing ID, status, priority, type, and title.
