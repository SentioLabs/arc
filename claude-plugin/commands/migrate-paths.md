---
description: Migrate legacy project configs to server-side workspace paths
argument-hint: [--dry-run] [--force]
---

Migrate legacy `~/.arc/projects/` configurations to server-side workspace path registrations.

```bash
arc migrate-paths              # Run migration
arc migrate-paths --dry-run    # Preview without making changes
arc migrate-paths --force      # Re-run even if already migrated
```

This is a one-time migration for users upgrading from older versions of arc that stored project-to-directory mappings only in local config files. After migration, the server handles path resolution, enabling multi-machine and container support.
