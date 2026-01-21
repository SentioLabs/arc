---
description: Manage workspaces
argument-hint: list|create|use|delete
---

Manage arc workspaces.

**List workspaces:**
```bash
arc workspace list
```

**Create workspace:**
```bash
arc workspace create my-project --path /path/to/project
```

**Set default workspace:**
```bash
arc workspace use my-project
```

**Delete workspace:**
```bash
arc workspace delete <id>
```

Each project typically has its own workspace. Use `arc init` in a project directory to create and configure a workspace automatically.
