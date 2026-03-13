---
description: Manage the arc CLI itself (update, release channel)
argument-hint: update|channel
---

Self-management commands for the arc CLI.

**Check for updates:**
```bash
arc self update --check
```

**Update to latest version:**
```bash
arc self update
arc self update --force    # Force reinstall even if up-to-date
```

**View or switch release channel:**
```bash
arc self channel                # Show current channel
arc self channel rc             # Switch to release candidates
arc self channel nightly        # Switch to nightly builds
arc self channel stable         # Switch back to stable
arc self channel nightly -y     # Switch without confirmation prompt
```

**Channels:**
- `stable` — Official releases (default)
- `rc` — Release candidates
- `nightly` — Daily builds from main branch

Major/minor version updates automatically create a database backup before installing.
