// Package main provides the onboard command, which gives AI agents and human
// users a quick orientation in the current arc workspace. It shows project
// statistics, in-progress work, ready items, and blocked issues so the user
// can decide what to work on next.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/spf13/cobra"
)

// onboardLimit is the default number of issues shown per category in onboard output.
const onboardLimit = 5

// errWorkspaceNotFound is returned when no workspace matches the current directory.
var errWorkspaceNotFound = errors.New("no workspace found for current directory")

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

If no workspace is configured locally but one exists on the server for this
directory, the local configuration will be automatically restored.

Run this at the start of a session to get context.`,
	RunE: runOnboard,
}

func init() {
	rootCmd.AddCommand(onboardCmd)
}

// projectConfig represents the .arc.json file structure
type projectConfig struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
}

// loadProjectConfig attempts to load .arc.json from the current directory
func loadProjectConfig() (*projectConfig, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	configPath := filepath.Join(cwd, ".arc.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg projectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// saveProjectConfig saves workspace info to .arc.json
func saveProjectConfig(workspaceID, workspaceName string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	configPath := filepath.Join(cwd, ".arc.json")

	cfg := projectConfig{
		WorkspaceID:   workspaceID,
		WorkspaceName: workspaceName,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, defaultFilePerm)
}

// findWorkspaceByPath queries the server for a workspace matching the current directory
func findWorkspaceByPath(c *client.Client) (*types.Workspace, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	workspaces, err := c.ListWorkspaces()
	if err != nil {
		return nil, err
	}

	for _, ws := range workspaces {
		if ws.Path == cwd {
			return ws, nil
		}
	}

	return nil, errWorkspaceNotFound
}

// tryRecoverWorkspace attempts to find and restore a workspace matching the
// current directory from the server. Returns the workspace ID if found, or
// an empty string when no matching workspace exists.
func tryRecoverWorkspace(c *client.Client) string {
	ws, err := findWorkspaceByPath(c)
	if err != nil {
		return ""
	}

	// Found workspace on server - restore local config
	_, _ = fmt.Println("## Workspace Recovery")
	_, _ = fmt.Println()
	_, _ = fmt.Printf("Found workspace **%s** on server for this directory.\n", ws.Name)
	_, _ = fmt.Println("Restoring local configuration...")
	_, _ = fmt.Println()

	// Save .arc.json
	if err := saveProjectConfig(ws.ID, ws.Name); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: could not save .arc.json: %v\n", err)
	} else {
		_, _ = fmt.Println("\u2713 Created .arc.json")
	}
	_, _ = fmt.Println()

	return ws.ID
}

//nolint:revive // function-length + CLI output: onboard prints many sequential lines
func runOnboard(cmd *cobra.Command, args []string) error {
	c, err := getClient()
	if err != nil {
		return fmt.Errorf("connect to server: %w", err)
	}

	var wsID string

	// Step 1: Try to get workspace ID from .arc.json
	projCfg, err := loadProjectConfig()
	if err == nil && projCfg.WorkspaceID != "" {
		wsID = projCfg.WorkspaceID
	}

	// Step 2: If no local config, try to find workspace by path from server
	if wsID == "" {
		wsID = tryRecoverWorkspace(c)
	}

	// Step 3: If still no workspace, suggest initialization
	if wsID == "" {
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
