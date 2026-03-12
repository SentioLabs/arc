// Package main provides the onboard command, which gives AI agents and human
// users a quick orientation in the current arc workspace. It shows project
// statistics, in-progress work, ready items, and blocked issues so the user
// can decide what to work on next.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/project"
	"github.com/spf13/cobra"
)

// onboardLimit is the default number of issues shown per category in onboard output.
const onboardLimit = 5

// onboardCmd displays essential workspace context for new sessions.
var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Get oriented with the current workspace",
	Long: `Display essential context about the current workspace and available work.

This command helps AI agents quickly understand:
- Current workspace configuration
- Open issues and their status
- Ready-to-work items
- Blocked issues

Run this at the start of a session to get context.`,
	RunE: runOnboard,
}

func init() {
	rootCmd.AddCommand(onboardCmd)
}

// resolveWorkspaceID resolves the workspace ID for the current directory
// via the server's workspace path resolution API.
func resolveWorkspaceID(c *client.Client) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	normalizedCwd := project.NormalizePath(cwd)

	resolution, err := c.ResolveWorkspaceByPath(normalizedCwd)
	if err != nil {
		return "", err
	}

	return resolution.WorkspaceID, nil
}

//nolint:revive // function-length + CLI output: onboard prints many sequential lines
func runOnboard(cmd *cobra.Command, args []string) error {
	c, err := getClient()
	if err != nil {
		return fmt.Errorf("connect to server: %w", err)
	}

	// Resolve workspace ID via server path resolution
	wsID, err := resolveWorkspaceID(c)
	if err != nil || wsID == "" {
		fmt.Println("# No Workspace Found")
		fmt.Println()
		fmt.Println("This directory is not configured for arc issue tracking.")
		fmt.Println()
		fmt.Println("**To initialize:**")
		fmt.Println("```bash")
		fmt.Println("arc init")
		fmt.Println("```")
		fmt.Println()
		fmt.Println("This will create a workspace and configure the project.")
		return nil
	}

	// Get workspace info
	ws, err := c.GetWorkspace(wsID)
	if err != nil {
		return fmt.Errorf("get workspace details: %w", err)
	}

	// Get statistics
	stats, err := c.GetStatistics(wsID)
	if err != nil {
		return fmt.Errorf("get statistics: %w", err)
	}

	// Get ready work (use default hybrid sort)
	readyIssues, err := c.GetReadyWork(wsID, onboardLimit, "")
	if err != nil {
		return fmt.Errorf("get ready work: %w", err)
	}

	// Get blocked issues
	blockedIssues, err := c.GetBlockedIssues(wsID, onboardLimit)
	if err != nil {
		return fmt.Errorf("get blocked issues: %w", err)
	}

	// Get in-progress issues
	inProgressIssues, err := c.ListIssues(wsID, client.ListIssuesOptions{
		Status: "in_progress",
		Limit:  onboardLimit,
	})
	if err != nil {
		return fmt.Errorf("get in-progress issues: %w", err)
	}

	// Output onboarding information
	_, _ = fmt.Println("# Workspace Onboarding")
	_, _ = fmt.Println()
	_, _ = fmt.Printf("**Workspace:** %s (%s)\n", ws.Name, ws.ID)
	_, _ = fmt.Printf("**Prefix:** %s\n", ws.Prefix)
	if ws.Description != "" {
		fmt.Printf("**Description:** %s\n", ws.Description)
	}
	fmt.Println()

	// Statistics
	fmt.Println("## Project Status")
	fmt.Println()
	fmt.Printf("| Metric | Count |\n")
	fmt.Printf("|--------|-------|\n")
	fmt.Printf("| Total Issues | %d |\n", stats.TotalIssues)
	fmt.Printf("| Open | %d |\n", stats.OpenIssues)
	fmt.Printf("| In Progress | %d |\n", stats.InProgressIssues)
	fmt.Printf("| Blocked | %d |\n", stats.BlockedIssues)
	fmt.Printf("| Ready to Work | %d |\n", stats.ReadyIssues)
	fmt.Printf("| Closed | %d |\n", stats.ClosedIssues)
	fmt.Println()

	// In-progress work
	if len(inProgressIssues) > 0 {
		fmt.Println("## Currently In Progress")
		fmt.Println()
		for _, issue := range inProgressIssues {
			fmt.Printf("- **[%s]** %s (P%d)\n", issue.ID, issue.Title, issue.Priority)
		}
		fmt.Println()
	}

	// Ready work
	if len(readyIssues) > 0 {
		fmt.Println("## Ready to Work")
		fmt.Println()
		fmt.Println("These issues have no blockers and can be started:")
		fmt.Println()
		for _, issue := range readyIssues {
			fmt.Printf("- **[%s]** %s (P%d, %s)\n", issue.ID, issue.Title, issue.Priority, issue.IssueType)
		}
		fmt.Println()
	} else {
		fmt.Println("## Ready to Work")
		fmt.Println()
		fmt.Println("No ready issues found. Check blocked issues or create new work.")
		fmt.Println()
	}

	// Blocked work
	if len(blockedIssues) > 0 {
		fmt.Println("## Blocked Issues")
		fmt.Println()
		for _, issue := range blockedIssues {
			blockers := strings.Join(issue.BlockedBy, ", ")
			fmt.Printf("- **[%s]** %s (blocked by: %s)\n", issue.ID, issue.Title, blockers)
		}
		fmt.Println()
	}

	// Quick commands
	fmt.Println("## Quick Commands")
	fmt.Println()
	fmt.Println("```bash")
	fmt.Println("arc ready                           # Find available work")
	fmt.Println("arc show <id>                       # View issue details")
	fmt.Println("arc update <id> --status in_progress  # Start working")
	fmt.Println("arc close <id>                      # Complete issue")
	fmt.Println("arc create \"title\" -p 2            # Create new issue")
	fmt.Println("```")
	fmt.Println()

	return nil
}
