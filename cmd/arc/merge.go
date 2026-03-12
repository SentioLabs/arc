package main

import (
	"fmt"
	"strings"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/spf13/cobra"
)

// projectMergeCmd merges one or more source projects into a target project.
var projectMergeCmd = &cobra.Command{
	Use:   "merge <source> [sources...]",
	Short: "Merge source projects into a target project",
	Long: `Merge one or more source projects into a target project.

All issues and plans from the source projects are moved into the target.
Source projects are deleted after a successful merge.

Projects can be specified by name or ID.

Examples:
  arc project merge --into target-proj source-proj
  arc project merge --into main-project project-a project-b`,
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

		// Resolve target project (name or ID)
		targetID, err := resolveProjectNameOrID(c, into)
		if err != nil {
			return fmt.Errorf("resolve target project %q: %w", into, err)
		}

		// Resolve source projects (names or IDs)
		var sourceIDs []string
		for _, src := range args {
			srcID, err := resolveProjectNameOrID(c, src)
			if err != nil {
				return fmt.Errorf("resolve source project %q: %w", src, err)
			}
			sourceIDs = append(sourceIDs, srcID)
		}

		result, err := c.MergeProjects(targetID, sourceIDs)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(result)
			return nil
		}

		fmt.Printf("Merged %d issues and %d plans into %s\n",
			result.IssuesMoved, result.PlansMoved, result.TargetProject.Name)
		if len(result.SourcesDeleted) > 0 {
			fmt.Printf("Deleted source projects: %s\n", strings.Join(result.SourcesDeleted, ", "))
		}

		return nil
	},
}

func init() {
	projectMergeCmd.Flags().String("into", "", "Target project name or ID (required)")
	projectCmd.AddCommand(projectMergeCmd)
}

// resolveProjectNameOrID resolves a project name or ID to a project ID.
// It first tries to get the project by ID; if that fails, it searches by name.
func resolveProjectNameOrID(c *client.Client, nameOrID string) (string, error) {
	// Try as ID first
	if proj, err := c.GetProject(nameOrID); err == nil {
		return proj.ID, nil
	}

	// Fall back to name lookup
	projects, err := c.ListProjects()
	if err != nil {
		return "", fmt.Errorf("list projects: %w", err)
	}

	for _, p := range projects {
		if p.Name == nameOrID {
			return p.ID, nil
		}
	}

	return "", fmt.Errorf("project %q not found", nameOrID)
}
