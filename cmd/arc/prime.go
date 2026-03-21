package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/sentiolabs/arc/internal/project"
	"github.com/spf13/cobra"
)

// stdinReadTimeout is the maximum time to wait for hook JSON on stdin.
const stdinReadTimeout = 2 * time.Second

// envFilePermissions is the file mode for Claude env files written by persistSessionID.
const envFilePermissions = 0o600

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
		// Read hook stdin and persist session ID if available
		sessionID := readHookStdin()
		if sessionID != "" {
			persistSessionID(sessionID)
		}

		// Also check env var (may have been set by a previous SessionStart hook)
		if sessionID == "" {
			sessionID = os.Getenv("ARC_SESSION_ID")
		}

		// Check if this project has arc configured via workspace path resolution
		cwd, err := os.Getwd()
		if err != nil {
			os.Exit(0)
		}
		normalizedCwd := project.NormalizePath(cwd)

		// Try server-based resolution first, fall back to legacy config
		arcConfigured := false
		c, clientErr := getClient()
		if clientErr == nil {
			if _, err := c.ResolveProjectByPath(normalizedCwd); err == nil {
				arcConfigured = true
			}
		}
		if !arcConfigured {
			// Fall back to legacy config check (works offline)
			arcHome := project.DefaultArcHome()
			if cfg, err := readLegacyConfig(arcHome, cwd); err != nil || cfg == nil {
				os.Exit(0)
			}
		}

		// Check for custom PRIME.md override
		localPrimePath := ".arc/PRIME.md"
		if content, err := os.ReadFile(localPrimePath); err == nil {
			_, _ = os.Stdout.Write(content)
			return
		}

		// Detect role from flag or env var
		role := primeRole
		if role == "" {
			role = os.Getenv("ARC_TEAMMATE_ROLE")
		}

		// Role-based output takes precedence over mode flags
		if role == "lead" {
			if err := outputTeamLeadContext(os.Stdout, sessionID); err != nil {
				os.Exit(0)
			}
			return
		} else if role != "" {
			if err := outputTeammateContext(os.Stdout, role, sessionID); err != nil {
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
		if err := outputPrimeContext(os.Stdout, mcpMode, sessionID); err != nil {
			os.Exit(0)
		}
	},
}

// hookInput represents the JSON payload sent by Claude Code to hooks via stdin.
type hookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	HookEventName  string `json:"hook_event_name"`
	Source         string `json:"source"`
}

// uuidPattern validates session IDs as UUIDs to prevent shell injection.
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// parseHookInput attempts to parse Claude Code hook JSON from the given reader.
// Returns empty string if not parseable or session_id is invalid.
// Separated from stdin detection for testability.
func parseHookInput(r io.Reader) string {
	// Read with a 2-second deadline to avoid hanging
	done := make(chan []byte, 1)
	go func() {
		data, err := io.ReadAll(r)
		if err != nil {
			done <- nil
			return
		}
		done <- data
	}()

	var data []byte
	select {
	case data = <-done:
	case <-time.After(stdinReadTimeout):
		return ""
	}

	if len(data) == 0 {
		return ""
	}

	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return ""
	}

	if !uuidPattern.MatchString(input.SessionID) {
		return ""
	}

	return input.SessionID
}

// readHookStdin attempts to read and parse Claude Code hook JSON from stdin.
// Returns empty string if stdin is a TTY, not parseable, or session_id is invalid.
func readHookStdin() string {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice != 0 {
		return "" // stdin is a TTY, not a pipe
	}
	return parseHookInput(os.Stdin)
}

// persistSessionID writes the session ID to CLAUDE_ENV_FILE for the session's
// Bash environment. Idempotent — replaces any existing ARC_SESSION_ID line.
func persistSessionID(sessionID string) {
	envFile := os.Getenv("CLAUDE_ENV_FILE")
	if envFile == "" || sessionID == "" {
		return
	}

	line := fmt.Sprintf("export ARC_SESSION_ID=%s\n", sessionID)

	// Read existing content to check for existing ARC_SESSION_ID
	existing, err := os.ReadFile(envFile) //nolint:gosec // envFile from trusted CLAUDE_ENV_FILE env var
	if err == nil {
		lines := strings.Split(string(existing), "\n")
		var filtered []string
		for _, l := range lines {
			if !strings.HasPrefix(l, "export ARC_SESSION_ID=") {
				filtered = append(filtered, l)
			}
		}
		content := strings.Join(filtered, "\n")
		if !strings.HasSuffix(content, "\n") && content != "" {
			content += "\n"
		}
		content += line
		//nolint:gosec // envFile from trusted CLAUDE_ENV_FILE env var
		_ = os.WriteFile(envFile, []byte(content), envFilePermissions)
		return
	}

	// File doesn't exist yet, create it
	//nolint:gosec // envFile from trusted CLAUDE_ENV_FILE env var
	_ = os.WriteFile(envFile, []byte(line), envFilePermissions)
}

// primeData holds template data for prime output templates.
type primeData struct {
	SessionID string
	Role      string // only used by teammate template
}

func init() {
	primeCmd.Flags().BoolVar(&primeFullMode, "full", false, "Force full CLI output")
	primeCmd.Flags().BoolVar(&primeMCPMode, "mcp", false, "Force MCP mode (minimal output)")
	primeCmd.Flags().StringVar(&primeRole, "role", "", "Teammate role (lead, frontend, backend, etc.)")
	rootCmd.AddCommand(primeCmd)
}

// outputPrimeContext outputs workflow context in markdown format
func outputPrimeContext(w io.Writer, mcpMode bool, sessionID string) error {
	if mcpMode {
		return outputMCPContext(w, sessionID)
	}
	return outputCLIContext(w, sessionID)
}

// outputMCPContext outputs minimal context for MCP users
func outputMCPContext(w io.Writer, sessionID string) error {
	return tmplMCP.Execute(w, primeData{SessionID: sessionID})
}

// outputTeamLeadContext outputs context for the team lead role.
// Includes the full CLI reference plus team sync protocol.
func outputTeamLeadContext(w io.Writer, sessionID string) error {
	if err := outputCLIContext(w, sessionID); err != nil {
		return err
	}
	return tmplTeamLead.Execute(w, primeData{SessionID: sessionID})
}

// outputTeammateContext outputs context for a specific teammate role.
// Concise — no full CLI reference or session close protocol.
// Teammates coordinate via TaskList and report to the team lead.
func outputTeammateContext(w io.Writer, role string, sessionID string) error {
	return tmplTeammate.Execute(w, primeData{SessionID: sessionID, Role: role})
}

// outputCLIContext outputs full CLI reference for non-MCP users
func outputCLIContext(w io.Writer, sessionID string) error {
	return tmplCLI.Execute(w, primeData{SessionID: sessionID})
}

// Prime output templates — each renders markdown context for a specific audience.
// Using text/template keeps the markdown readable without string concatenation.

var tmplMCP = template.Must(template.New("mcp").Parse(`# Arc Issue Tracker Active
{{- if .SessionID}}
> **Session**: ` + "`{{.SessionID}}`" + `
{{- end}}

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
{{- if .SessionID}}
> **Session**: ` + "`{{.SessionID}}`" + `
{{- end}}

You are the **{{.Role}}** teammate.

## Your Focus
- Work on issues labeled ` + "`teammate:{{.Role}}`" + `
- Use TaskList for real-time task coordination with the team
- Report completion to the team lead when done
`))

var tmplCLI = template.Must(template.New("cli").Parse(`# Arc Workflow Context

> **Context Recovery**: Run ` + "`arc prime`" + ` after compaction, clear, or new session
> Hooks auto-call this in Claude Code when project config detected
{{- if .SessionID}}
> **Session**: ` + "`{{.SessionID}}`" + `
{{- end}}

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
  - Use --stdin to read description from stdin (heredoc for multi-line):
    ` + "`arc create \"title\" --type=task --stdin <<'EOF'`" + `
    ` + "`description here`" + `
    ` + "`EOF`" + `
- ` + "`arc update <id> --status=in_progress`" + ` - Claim work
- ` + "`arc update <id> --assignee=username`" + ` - Assign to someone
- ` + "`arc update <id> --take`" + ` - Take issue for current AI session (sets session ID + in_progress)
- ` + "`arc update <id> --title=\"new title\"`" + ` - Update fields
- ` + "`arc update <id> --stdin <<'EOF'`" + ` - Update description via stdin heredoc
- ` + "`arc close <id>`" + ` - Mark complete
- ` + "`arc close <id1> <id2> ...`" + ` - Close multiple issues at once

### Labels
- ` + "`arc label list`" + ` - List all labels
- ` + "`arc label create <name> [--color=#hex] [--description=\"...\"]`" + ` - Create a label
- ` + "`arc label update <name> [--color=#hex] [--description=\"...\"]`" + ` - Update label metadata
- ` + "`arc label delete <name>`" + ` - Delete a label
- ` + "`arc create \"title\" --label=bug --label=urgent`" + ` - Create issue with labels
- ` + "`arc update <id> --label-add=critical --label-remove=stale`" + ` - Add/remove labels

### Dependencies & Blocking
- ` + "`arc dep add <issue> <depends-on>`" + ` - Add dependency (issue depends on depends-on)
- ` + "`arc blocked`" + ` - Show all blocked issues
- ` + "`arc show <id>`" + ` - See what's blocking/blocked by this issue

### Plans
- ` + "`arc plan create <file-path>`" + ` - Register ephemeral plan for review
- ` + "`arc plan show <plan-id>`" + ` - Show plan content, status, and comments
- ` + "`arc plan approve <plan-id>`" + ` - Approve plan
- ` + "`arc plan reject <plan-id>`" + ` - Reject plan
- ` + "`arc plan comments <plan-id>`" + ` - List review comments

### Project Health
- ` + "`arc stats`" + ` - Project statistics (open/closed/blocked counts)

## Common Workflows

**Starting work:**
` + "```bash" + `
arc ready           # Find available work
arc show <id>       # Review issue details
arc update <id> --take  # Take it (sets session ID + in_progress)
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
