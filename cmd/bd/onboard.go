package main

import (
	"fmt"
	"strings"

	"github.com/sentiolabs/beads-central/internal/client"
	"github.com/spf13/cobra"
)

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

func runOnboard(cmd *cobra.Command, args []string) error {
	c, err := getClient()
	if err != nil {
		return fmt.Errorf("connect to server: %w", err)
	}

	wsID, err := getWorkspaceID()
	if err != nil {
		return fmt.Errorf("get workspace: %w", err)
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

	// Get ready work
	readyIssues, err := c.GetReadyWork(wsID, 5)
	if err != nil {
		return fmt.Errorf("get ready work: %w", err)
	}

	// Get blocked issues
	blockedIssues, err := c.GetBlockedIssues(wsID, 5)
	if err != nil {
		return fmt.Errorf("get blocked issues: %w", err)
	}

	// Get in-progress issues
	inProgressIssues, err := c.ListIssues(wsID, client.ListIssuesOptions{
		Status: "in_progress",
		Limit:  5,
	})
	if err != nil {
		return fmt.Errorf("get in-progress issues: %w", err)
	}

	// Output onboarding information
	fmt.Println("# Workspace Onboarding")
	fmt.Println()
	fmt.Printf("**Workspace:** %s (%s)\n", ws.Name, ws.ID)
	fmt.Printf("**Prefix:** %s\n", ws.Prefix)
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
	fmt.Println("bd ready                           # Find available work")
	fmt.Println("bd show <id>                       # View issue details")
	fmt.Println("bd update <id> --status in_progress  # Start working")
	fmt.Println("bd close <id>                      # Complete issue")
	fmt.Println("bd create \"title\" -p 2            # Create new issue")
	fmt.Println("```")
	fmt.Println()

	return nil
}

