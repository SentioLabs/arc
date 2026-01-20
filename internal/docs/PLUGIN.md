# Claude Code Plugin Installation

This guide covers installing the arc Claude Code plugin for AI-assisted workflows.

## Contents

- [What the Plugin Provides](#what-the-plugin-provides)
- [Installation Methods](#installation-methods)
  - [Option A: Remote Marketplace](#option-a-remote-marketplace-recommended)
  - [Option B: Local Installation](#option-b-local-installation-development)
  - [Option C: Hooks Only](#option-c-hooks-only-minimal)
- [Verification](#verification)
- [Plugin Components](#plugin-components)
- [Troubleshooting](#troubleshooting)

## What the Plugin Provides

The arc plugin integrates with Claude Code to provide:

| Component | Purpose |
|-----------|---------|
| **SessionStart Hook** | Runs `arc prime` automatically when a session starts |
| **PreCompact Hook** | Runs `arc prime` before context compaction to preserve state |
| **Prompt Config** | Reminds Claude to run `arc onboard` in arc-enabled projects |
| **Skills** | Detailed documentation accessible via `/arc` skill |
| **Agent** | `arc-issue-tracker` agent for bulk operations via Task tool |

## Installation Methods

### Option A: Remote Marketplace (Recommended)

Install the plugin from the Sentio Labs marketplace:

```bash
# 1. Add the marketplace (one-time setup)
/plugin marketplace add sentiolabs/arc

# 2. Install the plugin
/plugin install arc

# 3. Restart Claude Code to activate hooks
```

**When to use**: Production use, easiest setup, automatic updates.

---

### Option B: Local Installation (Development)

Install from a local clone of the arc repository:

```bash
# 1. Clone the repository
git clone https://github.com/sentiolabs/arc.git
cd arc

# 2. Install the plugin from the local claude-plugin directory
/plugin install /path/to/arc/claude-plugin

# 3. Restart Claude Code to activate hooks
```

**Example with absolute path:**

```bash
# If you cloned to ~/projects/arc
/plugin install ~/projects/arc/claude-plugin
```

**When to use**:
- Developing or modifying the plugin
- Testing changes before publishing
- Contributing to arc
- Using a fork with custom modifications

**Note**: Local installations don't auto-update. Pull changes and reinstall to update.

---

### Option C: Hooks Only (Minimal)

If you only want the SessionStart/PreCompact hooks without the full plugin:

```bash
# Global installation (all projects)
arc setup claude

# Project-only installation
arc setup claude --project

# Verify installation
arc setup claude --check
```

**When to use**:
- Minimal footprint preferred
- Don't need skills or agents
- Just want automatic `arc prime` on session start

**What you lose**: Skills documentation, arc-issue-tracker agent, prompt configuration.

## Verification

After installation, verify the plugin is working:

```bash
# Check plugin is installed
/plugins

# Should show "arc" in the list

# Test the hook manually
arc prime

# Should output workflow context
```

**In a new session**, you should see `arc prime` output automatically if:
1. The plugin is installed
2. You're in a directory with `.arc.json` (arc-enabled project)

## Plugin Components

### Hooks

**SessionStart Hook**
- Triggers: When a new Claude Code session starts
- Action: Runs `arc prime` to inject workflow context
- Benefit: Claude immediately knows about arc and current project state

**PreCompact Hook**
- Triggers: Before context window compaction
- Action: Runs `arc prime` to preserve arc context
- Benefit: Arc workflow survives compaction events

### Skills

The `/arc` skill provides access to detailed documentation:
- Workflow guides
- Dependency type reference
- Decision matrices (arc vs TodoWrite)
- Resumability templates

Access via: `/arc` in Claude Code

### Agent

The `arc-issue-tracker` agent handles bulk operations:
- Creating epics with multiple child tasks
- Batch status updates
- Complex dependency setup

Use via Task tool when you need to run many arc commands without consuming main conversation context.

### Prompt Configuration

The plugin adds a prompt reminder:
> "When arc is available in a project, run 'arc onboard' at the start of work sessions..."

This ensures Claude proactively uses arc in enabled projects.

## Troubleshooting

### Plugin not showing in /plugins

1. Verify installation completed without errors
2. Restart Claude Code
3. Check plugin directory exists:
   ```bash
   ls ~/.claude/plugins/
   ```

### Hooks not firing

1. Verify you're in an arc-enabled project (`.arc.json` exists)
2. Check arc CLI is in PATH:
   ```bash
   which arc
   arc --version
   ```
3. Test manually:
   ```bash
   arc prime
   ```

### Local plugin changes not reflected

Local installations cache the plugin. To update:

```bash
# Uninstall
/plugin uninstall arc

# Reinstall from local path
/plugin install /path/to/arc/claude-plugin

# Restart Claude Code
```

### Marketplace vs Local conflict

If you have both installed, uninstall one:

```bash
# Remove marketplace version
/plugin uninstall arc

# Then install local version
/plugin install /path/to/arc/claude-plugin
```

Or vice versa.

## Updating

**Marketplace installation**: Updates automatically or via:
```bash
/plugin update arc
```

**Local installation**: Pull and reinstall:
```bash
cd /path/to/arc
git pull
/plugin uninstall arc
/plugin install /path/to/arc/claude-plugin
```

## Uninstalling

```bash
# Remove plugin
/plugin uninstall arc

# If using hooks-only installation
arc setup claude --uninstall
```
