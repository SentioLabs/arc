package main

import (
	"io"
	"os"
	"text/template"

	"github.com/spf13/cobra"
)

var (
	primeFullMode bool
	primeMCPMode  bool
	primeRole     string
)

var primeCmd = &cobra.Command{
	Use:   "prime",
	Short: "Output AI-optimized workflow context",
	Long: `Output essential Arc workflow context in AI-optimized markdown format.

Designed for Claude Code hooks (SessionStart, PreCompact) and manual use
in Codex CLI to prevent agents from forgetting arc workflow after compaction.

Modes:
- Default: Full CLI reference (~1-2k tokens)
- --mcp: Minimal output for MCP users (~50 tokens)
- --role=lead: Team lead context with sync protocol
- --role=<name>: Teammate context filtered by role

Role detection (in priority order):
1. --role flag
2. ARC_TEAMMATE_ROLE environment variable

Install hooks:
  arc setup claude          # Install SessionStart and PreCompact hooks
  arc setup codex           # Install repo-scoped Codex skill bundle

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
			os.Stdout.Write(content) //nolint:errcheck // best-effort output
			return
		}

		// Detect role from flag or env var
		role := primeRole
		if role == "" {
			role = os.Getenv("ARC_TEAMMATE_ROLE")
		}

		// Role-based output takes precedence over mode flags
		if role == "lead" {
			if err := outputTeamLeadContext(os.Stdout); err != nil {
				os.Exit(0)
			}
			return
		} else if role != "" {
			if err := outputTeammateContext(os.Stdout, role); err != nil {
				os.Exit(0)
			}
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
	primeCmd.Flags().StringVar(&primeRole, "role", "", "Teammate role (lead, frontend, backend, etc.)")
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
	return tmplMCP.Execute(w, nil)
}

// outputTeamLeadContext outputs context for the team lead role.
// Includes the full CLI reference plus team sync protocol.
func outputTeamLeadContext(w io.Writer) error {
	if err := outputCLIContext(w); err != nil {
		return err
	}
	return tmplTeamLead.Execute(w, nil)
}

// outputTeammateContext outputs context for a specific teammate role.
// Concise — no full CLI reference or session close protocol.
// Teammates coordinate via TaskList and report to the team lead.
func outputTeammateContext(w io.Writer, role string) error {
	return tmplTeammate.Execute(w, map[string]string{"Role": role})
}

// outputCLIContext outputs full CLI reference for non-MCP users
func outputCLIContext(w io.Writer) error {
	return tmplCLI.Execute(w, nil)
}

// Prime output templates — each renders markdown context for a specific audience.
// Using text/template keeps the markdown readable without string concatenation.

var tmplMCP = template.Must(template.New("mcp").Parse(`# Arc Issue Tracker Active

# 🚨 SESSION CLOSE PROTOCOL 🚨

Before saying "done": git status → git add → git commit → git push

## Core Rules
- Track strategic work in arc (multi-session, dependencies, discovered work)
- TodoWrite is fine for simple single-session linear tasks
- When in doubt, prefer arc—persistence you don't need beats lost context

Start: Check ` + "`arc ready`" + ` for available work.
`))

var tmplTeamLead = template.Must(template.New("team-lead").Parse(`
## Team Lead Protocol

You are the **team lead**. You coordinate teammates and verify their work.

### Sync Protocol
1. Use ` + "`arc team context <epic-id> --json`" + ` to check progress
2. After a teammate completes work, verify before closing arc issues
3. Close verified issues: ` + "`arc close <id> --reason \"completed by <teammate>\"`" + `

### Spawning Teammates
- Set ` + "`ARC_TEAMMATE_ROLE=<role>`" + ` env var when spawning each teammate
- Each teammate gets role-filtered context via ` + "`arc prime`" + `
- Teammates focus on issues labeled ` + "`teammate:<role>`" + `
`))

var tmplTeammate = template.Must(template.New("teammate").Parse(`# Arc Teammate Context

You are the **{{.Role}}** teammate.

## Your Focus
- Work on issues labeled ` + "`teammate:{{.Role}}`" + `
- Use TaskList for real-time task coordination with the team
- Report completion to the team lead when done
`))

var tmplCLI = template.Must(template.New("cli").Parse(`# Arc Workflow Context

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
- When in doubt, prefer arc—persistence you don't need beats lost context
- Git workflow: commit and push at session end
- Session management: check ` + "`arc ready`" + ` for available work

## Essential Commands

### Finding Work
- ` + "`arc ready`" + ` - Show issues ready to work (no blockers)
- ` + "`arc list --status=open`" + ` - All open issues
- ` + "`arc list --status=in_progress`" + ` - Your active work
- ` + "`arc show <id>`" + ` - Detailed issue view with dependencies

### Issue Types
- **bug**: Something is broken or not working as expected
- **feature**: New functionality or capability to add
- **task**: General work item, implementation step, or action to complete
- **epic**: Large initiative containing multiple related issues (use deps to link children)
- **chore**: Maintenance work that doesn't change functionality (refactoring, deps, cleanup, docs)

### Creating & Updating
- ` + "`arc create \"title\" --type=task|bug|feature|epic|chore --priority=2`" + ` - New issue
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

### Plans
- ` + "`arc plan set <issue-id> \"plan\"`" + ` - Set inline plan on issue
- ` + "`arc plan show <issue-id>`" + ` - Show all plans for issue (inline, parent, shared)
- ` + "`arc plan create \"title\"`" + ` - Create shared plan
- ` + "`arc plan link <plan-id> <issue>...`" + ` - Link plan to issues

### Project Health
- ` + "`arc stats`" + ` - Project statistics (open/closed/blocked counts)

## Common Workflows

**Starting work:**
` + "```bash" + `
arc ready           # Find available work
arc show <id>       # Review issue details
arc update <id> --status=in_progress  # Claim it
` + "```" + `

**Completing work:**
` + "```bash" + `
arc close <id1> <id2> ...   # Close all completed issues at once
git add . && git commit -m "..."  # Commit your changes
git push                    # Push to remote
` + "```" + `

**Creating dependent work:**
` + "```bash" + `
arc create "Implement feature X" --type=feature
arc create "Write tests for X" --type=task
arc dep add <tests-id> <feature-id>  # Tests depend on feature
` + "```" + `
`))
