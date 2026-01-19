package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var (
	primeFullMode bool
	primeMCPMode  bool
)

var primeCmd = &cobra.Command{
	Use:   "prime",
	Short: "Output AI-optimized workflow context",
	Long: `Output essential Arc workflow context in AI-optimized markdown format.

Designed for Claude Code hooks (SessionStart, PreCompact) to prevent
agents from forgetting arc workflow after context compaction.

Modes:
- Default: Full CLI reference (~1-2k tokens)
- --mcp: Minimal output for MCP users (~50 tokens)

Install hooks:
  arc setup claude          # Install SessionStart and PreCompact hooks

Workflow customization:
- Place a .arc/PRIME.md file to override the default output entirely.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if this project has arc configured
		if _, err := os.Stat(".arc.json"); os.IsNotExist(err) {
			// Not in a arc project - silent exit with success
			// This enables cross-platform hook integration
			os.Exit(0)
		}

		// Check for custom PRIME.md override
		localPrimePath := ".arc/PRIME.md"
		if content, err := os.ReadFile(localPrimePath); err == nil {
			fmt.Print(string(content))
			return
		}

		// Determine output mode
		mcpMode := primeMCPMode
		if primeFullMode {
			mcpMode = false
		}

		// Output workflow context
		if err := outputPrimeContext(os.Stdout, mcpMode); err != nil {
			os.Exit(0)
		}
	},
}

func init() {
	primeCmd.Flags().BoolVar(&primeFullMode, "full", false, "Force full CLI output")
	primeCmd.Flags().BoolVar(&primeMCPMode, "mcp", false, "Force MCP mode (minimal output)")
	rootCmd.AddCommand(primeCmd)
}

// outputPrimeContext outputs workflow context in markdown format
func outputPrimeContext(w io.Writer, mcpMode bool) error {
	if mcpMode {
		return outputMCPContext(w)
	}
	return outputCLIContext(w)
}

// outputMCPContext outputs minimal context for MCP users
func outputMCPContext(w io.Writer) error {
	context := `# Arc Issue Tracker Active

# 🚨 SESSION CLOSE PROTOCOL 🚨

Before saying "done": git status → git add → git commit → git push

## Core Rules
- Track strategic work in arc (multi-session, dependencies, discovered work)
- TodoWrite is fine for simple single-session linear tasks
- When in doubt, prefer arc—persistence you don't need beats lost context

Start: Check ` + "`arc ready`" + ` for available work.
`
	_, _ = fmt.Fprint(w, context)
	return nil
}

// outputCLIContext outputs full CLI reference for non-MCP users
func outputCLIContext(w io.Writer) error {
	context := `# Arc Workflow Context

> **Context Recovery**: Run ` + "`arc prime`" + ` after compaction, clear, or new session
> Hooks auto-call this in Claude Code when .arc.json detected

# 🚨 SESSION CLOSE PROTOCOL 🚨

**CRITICAL**: Before saying "done" or "complete", you MUST run this checklist:

` + "```" + `
[ ] 1. git status              (check what changed)
[ ] 2. git add <files>         (stage code changes)
[ ] 3. git commit -m "..."     (commit changes)
[ ] 4. git push                (push to remote)
` + "```" + `

**NEVER skip this.** Work is not done until pushed.

## Core Rules
- Track strategic work in arc (multi-session, dependencies, discovered work)
- Use ` + "`arc create`" + ` for issues, TodoWrite for simple single-session execution
- When in doubt, prefer bd—persistence you don't need beats lost context
- Git workflow: commit and push at session end
- Session management: check ` + "`arc ready`" + ` for available work

## Essential Commands

### Finding Work
- ` + "`arc ready`" + ` - Show issues ready to work (no blockers)
- ` + "`arc list --status=open`" + ` - All open issues
- ` + "`arc list --status=in_progress`" + ` - Your active work
- ` + "`arc show <id>`" + ` - Detailed issue view with dependencies

### Creating & Updating
- ` + "`arc create \"title\" --type=task|bug|feature --priority=2`" + ` - New issue
  - Priority: 0-4 (0=critical, 2=medium, 4=backlog)
- ` + "`arc update <id> --status=in_progress`" + ` - Claim work
- ` + "`arc update <id> --assignee=username`" + ` - Assign to someone
- ` + "`arc update <id> --title=\"new title\"`" + ` - Update fields
- ` + "`arc close <id>`" + ` - Mark complete
- ` + "`arc close <id1> <id2> ...`" + ` - Close multiple issues at once

### Dependencies & Blocking
- ` + "`arc dep add <issue> <depends-on>`" + ` - Add dependency (issue depends on depends-on)
- ` + "`arc blocked`" + ` - Show all blocked issues
- ` + "`arc show <id>`" + ` - See what's blocking/blocked by this issue

### Project Health
- ` + "`arc stats`" + ` - Project statistics (open/closed/blocked counts)

## Common Workflows

**Starting work:**
` + "```bash" + `
bd ready           # Find available work
bd show <id>       # Review issue details
bd update <id> --status=in_progress  # Claim it
` + "```" + `

**Completing work:**
` + "```bash" + `
bd close <id1> <id2> ...    # Close all completed issues at once
git add . && git commit -m "..."  # Commit your changes
git push                    # Push to remote
` + "```" + `

**Creating dependent work:**
` + "```bash" + `
bd create "Implement feature X" --type=feature
bd create "Write tests for X" --type=task
bd dep add <tests-id> <feature-id>  # Tests depend on feature
` + "```" + `
`
	_, _ = fmt.Fprint(w, context)
	return nil
}
