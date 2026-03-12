package main

import (
	"fmt"
	"strings"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/spf13/cobra"
)

// workspaceMergeCmd merges one or more source workspaces into a target workspace.
var workspaceMergeCmd = &cobra.Command{
	Use:   "merge <source> [sources...]",
	Short: "Merge source workspaces into a target workspace",
	Long: `Merge one or more source workspaces into a target workspace.

All issues and plans from the source workspaces are moved into the target.
Source workspaces are deleted after a successful merge.

Workspaces can be specified by name or ID.

Examples:
  arc workspace merge --into target-ws source-ws
  arc workspace merge --into main-project project-a project-b`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		into, _ := cmd.Flags().GetString("into")
		if into == "" {
			return fmt.Errorf("--into flag is required")
		}

		c, err := getClient()
		if err != nil {
			return err
		}

		// Resolve target workspace (name or ID)
		targetID, err := resolveWorkspaceNameOrID(c, into)
		if err != nil {
			return fmt.Errorf("resolve target workspace %q: %w", into, err)
		}

		// Resolve source workspaces (names or IDs)
		var sourceIDs []string
		for _, src := range args {
			srcID, err := resolveWorkspaceNameOrID(c, src)
			if err != nil {
				return fmt.Errorf("resolve source workspace %q: %w", src, err)
			}
			sourceIDs = append(sourceIDs, srcID)
		}

		result, err := c.MergeWorkspaces(targetID, sourceIDs)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(result)
			return nil
		}

		fmt.Printf("Merged %d issues and %d plans into %s\n",
			result.IssuesMoved, result.PlansMoved, result.TargetWorkspace.Name)
		if len(result.SourcesDeleted) > 0 {
			fmt.Printf("Deleted source workspaces: %s\n", strings.Join(result.SourcesDeleted, ", "))
		}

		return nil
	},
}

func init() {
	workspaceMergeCmd.Flags().String("into", "", "Target workspace name or ID (required)")
	workspaceCmd.AddCommand(workspaceMergeCmd)
}

// resolveWorkspaceNameOrID resolves a workspace name or ID to a workspace ID.
// It first tries to get the workspace by ID; if that fails, it searches by name.
func resolveWorkspaceNameOrID(c *client.Client, nameOrID string) (string, error) {
	// Try as ID first
	if ws, err := c.GetWorkspace(nameOrID); err == nil {
		return ws.ID, nil
	}

	// Fall back to name lookup
	workspaces, err := c.ListWorkspaces()
	if err != nil {
		return "", fmt.Errorf("list workspaces: %w", err)
	}

	for _, ws := range workspaces {
		if ws.Name == nameOrID {
			return ws.ID, nil
		}
	}

	return "", fmt.Errorf("workspace %q not found", nameOrID)
}
