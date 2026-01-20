---
description: Show detailed information about an issue
argument-hint: <issue-id>
---

Show details for an arc issue.

If the issue ID is missing, ask the user for it or suggest running `arc list` to find issues.

Run `arc show <id>` to display:
- Issue ID, title, status
- Priority and type
- Description
- Dependencies (blocking/blocked by)
- Labels and comments

**For epics:** The show command also displays child issues linked via parent-child dependencies, helping you see the full scope of work in an epic.
