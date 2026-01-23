---
description: Search and browse arc documentation
---

## Two-Step Workflow

1. **Search** to find which topic has the information: `arc docs search "query"`
2. **Read** the full topic for details: `arc docs <topic>`

### Example

```bash
# Step 1: Search to find where the info is
$ arc docs search "create issue"
Results for "create issue":
1. [workflows] Discovery and Issue Creation
   Discovery and Issue Creation **When encountering new work...
2. [workflows] Creating Issues During Work
   ...

# Step 2: Read the workflows topic for full details
$ arc docs workflows
```

The search results show `[topic]` in brackets - use that topic name with `arc docs <topic>` to read the full section.

## Search Command

```bash
arc docs search "blocks vs related"     # dependency types
arc docs search "todowrite vs arc"      # boundaries
arc docs search "compaction notes"      # resumability
arc docs search "session start"         # workflows
```

Fuzzy matching handles typos - "dependncy" finds "dependency" docs.

**Flags:**
- `-n, --limit` - Max results (default: 5)
- `--exact` - Disable fuzzy matching
- `-v, --verbose` - Show relevance scores

## Available Topics

| Command | Purpose |
|---------|---------|
| `arc docs` | Overview of all topics |
| `arc docs workflows` | Step-by-step checklists |
| `arc docs dependencies` | Dependency types and when to use each |
| `arc docs boundaries` | When to use arc vs TodoWrite |
| `arc docs resumability` | Writing notes that survive compaction |
| `arc docs plugin` | Claude Code plugin installation |
