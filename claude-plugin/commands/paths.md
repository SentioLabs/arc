---
description: Manage workspace path registrations for a project
argument-hint: [list|add|remove] [--all]
---

Manage filesystem paths associated with the current project. Paths link directories to projects for automatic project resolution.

**List paths for current project:**
```bash
arc paths
arc paths list
```

**List paths across all projects:**
```bash
arc paths list --all
```

**Register a path:**
```bash
arc paths add <dir>
arc paths add <dir> --label "my laptop" --hostname myhost
```

**Unregister a path:**
```bash
arc paths remove <path-or-id>
```

**Flags for `add`:**
- `--label`: Human-readable label for the path
- `--hostname`: Override auto-detected hostname

Paths are registered automatically by `arc init`. Use `arc paths add` to register additional directories (e.g., worktrees, secondary checkouts).
