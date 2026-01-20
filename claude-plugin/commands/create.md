---
description: Create a new issue interactively
argument-hint: [title] [--type TYPE] [--priority N] [--parent EPIC-ID]
---

Create a new arc issue. If arguments are provided:
- $1: Issue title
- --type: Issue type (bug, feature, task, epic, chore)
- --priority: Priority (0-4, where 0=critical, 4=backlog)
- --parent: Parent epic ID (creates child issue)

If arguments are missing, ask the user for:
1. Issue title (required)
2. Issue type (default: task)
3. Priority (default: 2)
4. Description (optional)
5. Parent epic (optional, for child issues)

**Create a standalone issue:**
```bash
arc create "Fix login bug" --type bug --priority 1
arc create "Add dark mode" --type feature
```

**Create an epic with children:**
```bash
# Create the epic first
arc create "User Authentication System" --type epic --priority 1

# Create child tasks under the epic
arc create "Implement login flow" --type task --parent <epic-id>
arc create "Add password reset" --type task --parent <epic-id>
arc create "Add OAuth support" --type feature --parent <epic-id>
```

Child issues automatically get a parent-child dependency to the epic.

Optionally ask if this issue should be linked to another issue using `arc dep add` (blocks, related, discovered-from).
