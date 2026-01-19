package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var quickstartCmd = &cobra.Command{
	Use:   "quickstart",
	Short: "Quick start guide for bd",
	Long:  `Display a quick start guide for using bd (beads-central).`,
	Run:   runQuickstart,
}

func init() {
	rootCmd.AddCommand(quickstartCmd)
}

func runQuickstart(cmd *cobra.Command, args []string) {
	fmt.Print(`# Beads-Central Quick Start

## What is bd?

bd (beads-central) is a central issue tracking system designed for AI-assisted
coding workflows. It helps you track tasks, bugs, and features across projects
with a simple CLI.

## Core Concepts

- **Workspace**: A project container (like a repo). Each workspace has its own issues.
- **Issue**: A trackable unit of work (task, bug, feature, epic, chore)
- **Dependency**: Issues can block or depend on other issues
- **Status**: open, in_progress, blocked, deferred, closed
- **Priority**: 0 (critical) to 4 (backlog), default is 2 (medium)

## Basic Workflow

### 1. Find work
` + "```bash" + `
bd ready              # Show issues ready to work (no blockers)
bd list               # List all issues
bd list --status open # Filter by status
` + "```" + `

### 2. Start working
` + "```bash" + `
bd show <id>                          # View issue details
bd update <id> --status in_progress   # Claim the issue
` + "```" + `

### 3. Complete work
` + "```bash" + `
bd close <id>                # Mark issue as done
bd close <id> --reason "..." # Close with a note
` + "```" + `

### 4. Create new issues
` + "```bash" + `
bd create "Fix login bug" --type bug --priority 1
bd create "Add dark mode" --type feature
bd create "Write tests" --type task
` + "```" + `

### 5. Manage dependencies
` + "```bash" + `
bd dep add <issue> <depends-on>  # issue depends on depends-on
bd blocked                        # Show all blocked issues
` + "```" + `

## Key Commands Reference

| Command | Description |
|---------|-------------|
| bd init | Initialize workspace in current directory |
| bd ready | Show unblocked issues |
| bd list | List all issues |
| bd show <id> | Show issue details |
| bd create <title> | Create new issue |
| bd update <id> | Update issue fields |
| bd close <id> | Close an issue |
| bd blocked | Show blocked issues |
| bd stats | Show workspace statistics |
| bd onboard | Get workspace orientation |

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

1. Run ` + "`bd onboard`" + ` at session start to get context
2. Use ` + "`bd ready`" + ` to find available work
3. Always close issues when work is complete
4. Create issues for discovered work during sessions
5. Push all changes before ending session

## More Help

- ` + "`bd --help`" + ` - Full command list
- ` + "`bd <command> --help`" + ` - Command-specific help
`)
}
