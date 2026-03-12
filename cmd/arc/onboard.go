// Package main provides the onboard command, which gives AI agents and human
// users a quick orientation in the current arc workspace. It shows project
// statistics, in-progress work, ready items, and blocked issues so the user
// can decide what to work on next.
package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/project"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/spf13/cobra"
)

// onboardLimit is the default number of issues shown per category in onboard output.
const onboardLimit = 5

// errProjectNotFound is returned when no project matches the current directory.
var errProjectNotFound = errors.New("no project found for current directory")

// onboardCmd displays essential project context for new sessions.
var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Get oriented with the current project",
	Long: `Display essential context about the current project and available work.

This command helps AI agents quickly understand:
- Current project configuration
- Open issues and their status
- Ready-to-work items
- Blocked issues

If no project is configured locally but one exists on the server for this
directory, the local configuration will be automatically restored.

Run this at the start of a session to get context.`,
	RunE: runOnboard,
}

func init() {
	rootCmd.AddCommand(onboardCmd)
}

// loadOnboardConfig attempts to load project config from legacy ~/.arc/projects/ configs.
func loadOnboardConfig() (*legacyProjectConfig, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	arcHome := project.DefaultArcHome()
	cfg, err := readLegacyConfig(arcHome, cwd)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("no legacy config found")
	}
	return cfg, nil
}

// findProjectByPath queries the server for a project matching the current directory.
func findProjectByPath(c *client.Client) (*types.Project, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Try server-side resolution via workspace paths
	if res, resolveErr := c.ResolveProjectByPath(cwd); resolveErr == nil && res.ProjectID != "" {
		if proj, getErr := c.GetProject(res.ProjectID); getErr == nil {
			return proj, nil
		}
	}

	// Also try with normalized (symlink-resolved) path
	normalizedCwd := project.NormalizePath(cwd)
	if normalizedCwd != cwd {
		if res, resolveErr := c.ResolveProjectByPath(normalizedCwd); resolveErr == nil && res.ProjectID != "" {
			if proj, getErr := c.GetProject(res.ProjectID); getErr == nil {
				return proj, nil
			}
		}
	}

	return nil, errProjectNotFound
}

// tryRecoverProject attempts to find and restore a project matching the
// current directory from the server. Returns the project ID if found, or
// an empty string when no matching project exists.
func tryRecoverProject(c *client.Client) string {
	proj, err := findProjectByPath(c)
	if err != nil {
		return ""
	}

	// Found project on server
	_, _ = fmt.Println("## Project Recovery")
	_, _ = fmt.Println()
	_, _ = fmt.Printf("Found project **%s** on server for this directory.\n", proj.Name)
	_, _ = fmt.Println()

	return proj.ID
}

//nolint:revive // function-length + CLI output: onboard prints many sequential lines
func runOnboard(cmd *cobra.Command, args []string) error {
	c, err := getClient()
	if err != nil {
		return fmt.Errorf("connect to server: %w", err)
	}

	var wsID string

	// Step 1: Try to find project by path from server
	wsID = tryRecoverProject(c)

	// Step 2: If no server match, check legacy config
	if wsID == "" {
		projCfg, cfgErr := loadOnboardConfig()
		if cfgErr == nil && projCfg.WorkspaceID != "" {
			wsID = projCfg.WorkspaceID
		}
	}

	// Step 3: If still no project, suggest initialization
	if wsID == "" {
		fmt.Println("# No Project Found")
		fmt.Println()
		fmt.Println("This directory is not configured for arc issue tracking.")
		fmt.Println()
		fmt.Println("**To initialize:**")
		fmt.Println("```bash")
		fmt.Println("arc init")
		fmt.Println("```")
		fmt.Println()
		fmt.Println("This will create a project and configure the directory.")
		return nil
	}

	// Get project info
	proj, err := c.GetProject(wsID)
	if err != nil {
		return fmt.Errorf("get project details: %w", err)
	}

	// Get statistics
	stats, err := c.GetProjectStats(wsID)
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
	_, _ = fmt.Println("# Project Onboarding")
	_, _ = fmt.Println()
	_, _ = fmt.Printf("**Project:** %s (%s)\n", proj.Name, proj.ID)
	_, _ = fmt.Printf("**Prefix:** %s\n", proj.Prefix)
	if proj.Description != "" {
		fmt.Printf("**Description:** %s\n", proj.Description)
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
