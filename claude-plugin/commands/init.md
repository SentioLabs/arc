---
description: Initialize arc in the current project
argument-hint: [workspace-name]
---

Initialize arc in the current directory.

```bash
arc init                        # Use directory name as workspace
arc init my-project             # Custom workspace name
arc init --prefix cxsh          # Custom issue prefix (e.g., cxsh-0b7w)
arc init my-project -p cxsh     # Both custom name and prefix
```

This command:
1. Creates a workspace on the arc server
2. Saves project config to `~/.arc/projects/`
3. Creates AGENTS.md with workflow instructions

**Flags:**
- `--prefix`, `-p`: Custom issue prefix basename (alphanumeric, max 10 chars). Gets normalized (lowercased, special chars stripped) and combined with a hash suffix for uniqueness.
- `--description`, `-d`: Workspace description
- `--quiet`, `-q`: Suppress output

**Prerequisites:**
- Arc server must be running (`arc server start`)
