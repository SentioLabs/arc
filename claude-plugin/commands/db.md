---
description: Database management commands
argument-hint: backup [--db PATH]
---

Manage the arc database.

**Create a backup:**
```bash
arc db backup
arc db backup --db /path/to/data.db
```

Creates a timestamped, gzip-compressed backup next to the database file:
```
~/.arc/data.db.20260312_155850.gz
```

Backups are also created automatically before major/minor version updates via `arc self update`.
