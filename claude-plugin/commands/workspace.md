---
description: Manage projects
argument-hint: list|create|use|delete
---

Manage arc projects.

**List projects:**
```bash
arc project list
```

**Create project:**
```bash
arc project create my-project --path /path/to/project
```

**Set default project:**
```bash
arc project use my-project
```

**Delete project:**
```bash
arc project delete <id>
```

Each directory typically has its own project. Use `arc init` in a project directory to create and configure a project automatically.
