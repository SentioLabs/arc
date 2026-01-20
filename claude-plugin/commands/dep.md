---
description: Manage dependencies between issues
argument-hint: add|remove <issue> <depends-on> [--type TYPE]
---

Manage issue dependencies with `arc dep`.

**Add dependency:**
```bash
arc dep add <issue> <depends-on>              # Default: blocks
arc dep add <issue> <depends-on> -t blocks    # Explicit type
arc dep add <issue> <depends-on> -t related   # Related link
```

**Remove dependency:**
```bash
arc dep remove <issue> <depends-on>
```

**Dependency types:**

| Type | Description | Use case |
|------|-------------|----------|
| `blocks` | Issue A blocks issue B | B can't start until A is done |
| `parent-child` | Hierarchical relationship | Epic contains tasks |
| `related` | Loose association | Related work |
| `discovered-from` | Found during other work | Bug found while working on feature |

**Epic relationships:**
When creating child issues with `--parent`, a parent-child dependency is automatically created. You can also manually link:
```bash
arc dep add <child-id> <epic-id> -t parent-child
```

When an issue is blocked, it won't appear in `arc ready` until its blockers are closed.
