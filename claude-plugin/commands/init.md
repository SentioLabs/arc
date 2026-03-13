---
description: Initialize arc in the current project
argument-hint: [project-name]
---

Initialize arc in the current directory.

```bash
arc init                        # Use directory name as project
arc init my-project             # Custom project name
arc init --prefix cxsh          # Custom issue prefix (e.g., cxsh-0b7w)
arc init my-project -p cxsh     # Both custom name and prefix
```

This command:
1. Creates a project on the arc server (or connects to existing)
2. Registers the current directory as a workspace path on the server
3. Saves project config to `~/.arc/projects/`
4. Creates AGENTS.md with workflow instructions

**Flags:**
- `--prefix`, `-p`: Custom issue prefix basename (alphanumeric, max 10 chars). Gets normalized (lowercased, special chars stripped) and combined with a hash suffix for uniqueness.
- `--description`, `-d`: Project description
- `--quiet`, `-q`: Suppress output

**Prerequisites:**
- Arc server must be running (`arc server start`)
