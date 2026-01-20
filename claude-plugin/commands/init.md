---
description: Initialize arc in the current project
argument-hint: [workspace-name]
---

Initialize arc in the current directory.

```bash
arc init                    # Use directory name as workspace
arc init my-project         # Custom workspace name
arc init --prefix mp        # Custom issue prefix
```

This command:
1. Creates a workspace on the arc server
2. Saves workspace config to `.arc.json`
3. Creates AGENTS.md with workflow instructions
4. Sets up Claude Code hooks (unless --skip-claude)

**Prerequisites:**
- Arc server must be running (`arc server start`)
