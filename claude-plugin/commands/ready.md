---
description: Find ready-to-work tasks with no blockers
---

Run `arc ready` to find tasks that are ready to work on (no blocking dependencies).

Present the results to the user showing:
- Issue ID
- Title
- Priority
- Issue type

If there are ready tasks, ask the user which one they'd like to work on. If they choose one, run `arc update <id> --status in_progress` to claim it.

If there are no ready tasks, suggest checking `arc blocked` or creating a new issue with `arc create`.
