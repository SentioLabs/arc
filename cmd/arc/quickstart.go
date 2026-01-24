package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var quickstartCmd = &cobra.Command{
	Use:   "quickstart",
	Short: "Quick start guide for arc",
	Long:  `Display a quick start guide for using arc.`,
	Run:   runQuickstart,
}

func init() {
	rootCmd.AddCommand(quickstartCmd)
}

func runQuickstart(cmd *cobra.Command, args []string) {
	fmt.Print(`# Arc Quick Start

## What is arc?

arc is a central issue tracking system designed for AI-assisted
coding workflows. It helps you track tasks, bugs, and features across projects
with a simple CLI.

## Core Concepts

- **Workspace**: A project container (like a repo). Each workspace has its own issues.
- **Issue**: A trackable unit of work (task, bug, feature, epic, chore)
- **Dependency**: Issues can block or depend on other issues
- **Status**: open, in_progress, blocked, deferred, closed
- **Priority**: 0 (critical) to 4 (backlog), default is 2 (medium)

## Claude Code Integration (Recommended)

For the best AI-assisted workflow, install the arc plugin:

### Option A: Plugin Marketplace

` + "```bash" + `
# In Claude Code, first add the marketplace
/plugin marketplace add sentiolabs/arc

# Then install the plugin
/plugin install arc

# Restart Claude Code
` + "```" + `

### Option B: CLI Hooks Only

` + "```bash" + `
arc setup claude            # Global installation
arc setup claude --project  # Project-only installation
arc setup claude --check    # Verify installation
` + "```" + `

### What the Plugin Provides

| Component | Benefit |
|-----------|---------|
| **SessionStart Hook** | Auto-runs ` + "`arc prime`" + ` on session start |
| **PreCompact Hook** | Preserves context before compaction |
| **Prompt Config** | Reminds Claude to run ` + "`arc onboard`" + ` |
| **Skills** | Detailed guides for arc workflows |
| **Agent** | Bulk operations via Task tool |

## Codex CLI Integration

Codex CLI uses repo-scoped skills under .codex/skills. Install the arc skill bundle:

` + "```bash" + `
arc setup codex
` + "```" + `

**Notes:**
- Codex CLI does not support lifecycle hooks.
- Run ` + "`arc onboard`" + ` at session start.
- Run ` + "`arc prime`" + ` after compaction or if workflow context is stale.

## Basic Workflow

### 1. Find work
` + "```bash" + `
arc ready              # Show issues ready to work (no blockers)
arc list               # List all issues
arc list --status open # Filter by status
` + "```" + `

### 2. Start working
` + "```bash" + `
arc show <id>                          # View issue details
arc update <id> --status in_progress   # Claim the issue
` + "```" + `

### 3. Complete work
` + "```bash" + `
arc close <id>                # Mark issue as done
arc close <id> --reason "..." # Close with a note
` + "```" + `

### 4. Create new issues
` + "```bash" + `
arc create "Fix login bug" --type bug --priority 1
arc create "Add dark mode" --type feature
arc create "Write tests" --type task
` + "```" + `

### 5. Manage dependencies
` + "```bash" + `
arc dep add <issue> <depends-on>  # issue depends on depends-on
arc blocked                        # Show all blocked issues
` + "```" + `

## Key Commands Reference

| Command | Description |
|---------|-------------|
| arc init | Initialize workspace in current directory |
| arc ready | Show unblocked issues |
| arc list | List all issues |
| arc show <id> | Show issue details |
| arc create <title> | Create new issue |
| arc update <id> | Update issue fields |
| arc close <id> | Close an issue |
| arc blocked | Show blocked issues |
| arc stats | Show workspace statistics |
| arc onboard | Get workspace orientation |
| arc setup claude | Install Claude Code hooks |
| arc setup codex | Install Codex repo skill bundle |

## Priority Levels

| Level | Meaning |
|-------|---------|
| P0 | Critical - drop everything |
| P1 | High - do this sprint |
| P2 | Medium - normal priority (default) |
| P3 | Low - nice to have |
| P4 | Backlog - someday |

## Issue Types

- **task**: General work item
- **bug**: Something broken
- **feature**: New functionality
- **epic**: Large feature spanning multiple issues
- **chore**: Maintenance, refactoring, etc.

## Tips for AI Agents

1. Run ` + "`arc onboard`" + ` at session start to get context
2. Use ` + "`arc ready`" + ` to find available work
3. Always close issues when work is complete
4. Create issues for discovered work during sessions
5. Push all changes before ending session

## More Help

- ` + "`arc --help`" + ` - Full command list
- ` + "`arc <command> --help`" + ` - Command-specific help
- ` + "`arc prime`" + ` - Full workflow context for AI agents
`)
}
