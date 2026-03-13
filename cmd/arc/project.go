package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var projectRenameCmd = &cobra.Command{
	Use:   "rename <new-name>",
	Short: "Rename the current project",
	Long: `Rename the current project to a new name.

The project is resolved from the current working directory.
Use --project to specify a different project by ID.

Examples:
  arc project rename my-new-name
  arc project rename --project proj-a1b2 my-new-name`,
	Args: cobra.ExactArgs(1),
	RunE: runProjectRename,
}

func init() {
	projectCmd.AddCommand(projectRenameCmd)
}

func runProjectRename(_ *cobra.Command, args []string) error {
	newName := args[0]

	// Resolve current project
	wsID, _, _, err := resolveProject()
	if err != nil {
		return fmt.Errorf("resolve project: %w", err)
	}

	c, err := getClient()
	if err != nil {
		return fmt.Errorf("connect to server: %w", err)
	}

	// Get current project to show the old name
	proj, err := c.GetProject(wsID)
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}

	oldName := proj.Name
	if oldName == newName {
		fmt.Println("Project name is already", newName)
		return nil
	}

	updated, err := c.UpdateProject(wsID, map[string]any{"name": newName})
	if err != nil {
		return fmt.Errorf("rename project: %w", err)
	}

	if outputJSON {
		outputResult(updated)
	} else {
		fmt.Printf("Renamed project: %s → %s\n", oldName, updated.Name)
	}

	return nil
}
