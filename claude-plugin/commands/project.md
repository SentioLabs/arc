---
description: Manage projects
argument-hint: list|create|delete|rename|merge
---

Manage arc projects.

**List projects:**
```bash
arc project list
```

**Create project:**
```bash
arc project create my-project
```

**Delete project:**
```bash
arc project delete <id>
```

**Rename project:**
```bash
arc project rename <new-name>
arc project rename --project <id> <new-name>
```

**Merge projects:**
```bash
arc project merge --into <target> <source> [sources...]
```
Merges all issues and plans from source projects into the target, then deletes source projects. Projects can be specified by name or ID.

Each directory typically has its own project. Use `arc init` in a project directory to create and configure a project automatically. Use `arc paths add` to register additional directories to an existing project.
