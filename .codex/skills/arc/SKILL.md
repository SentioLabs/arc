---
name: arc
description: Repo-scoped Arc issue tracking workflow for Codex CLI
---

# Arc Issue Tracker (Codex)

Track complex, multi-session work with a central issue tracking system.

## Session start

Codex CLI has no lifecycle hooks. Do this at the start of each session:

```bash
arc onboard        # Recover workspace, load context
arc prime          # Refresh workflow context if stale or after compaction
```

## When to use Arc vs TodoWrite

| Use Arc | Use TodoWrite |
|---------|---------------|
| Multi-session work | Single-session tasks |
| Complex dependencies | Linear task lists |
| Discovered work patterns | Simple checklists |
| Work needing audit trail | Quick, disposable lists |

Rule of thumb: when in doubt, prefer arc - persistence you don't need beats lost context.

## Essential commands

- `arc ready` - Find unblocked work
- `arc create` - Create issues
- `arc update` - Update status/fields
- `arc close` - Complete work
- `arc show` - View details
- `arc dep` - Manage dependencies

## Documentation lookup

Two-step workflow:

1. `arc docs search "query"`
2. `arc docs <topic>`

## Session completion

Follow the **Landing the Plane** protocol in `AGENTS.md` before ending a session.

## Notes

- Repo-scoped skill source lives under `.codex/skills/arc/`.
- If `.arc.json` is missing but the workspace exists on the server, `arc onboard` will recover it.
