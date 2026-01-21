---
description: Setup Claude Code integration
argument-hint: claude [--project] [--check] [--remove]
---

Setup Claude Code hooks for arc.

**Install hooks:**
```bash
arc setup claude            # Global installation
arc setup claude --project  # Project-only installation
```

**Check/remove:**
```bash
arc setup claude --check    # Verify installation
arc setup claude --remove   # Remove hooks
```

This installs SessionStart and PreCompact hooks that run `arc prime` to provide workflow context.

**Note:** If you installed the arc Claude plugin, hooks are already configured in the plugin. You only need `arc setup claude` if using arc without the plugin.
