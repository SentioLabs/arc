// Label management commands for creating, listing, updating, and deleting
// global labels. Labels are shared across all projects and can be associated
// with issues for categorization and filtering.
package main

import (
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// labelCmd is the parent command for label management.
// Subcommands: list, create, update, delete.
var labelCmd = &cobra.Command{
	Use:   "label",
	Short: "Manage labels",
}

func init() {
	labelCmd.AddCommand(labelListCmd)
	labelCmd.AddCommand(labelCreateCmd)
	labelCmd.AddCommand(labelUpdateCmd)
	labelCmd.AddCommand(labelDeleteCmd)
}

// labelListCmd lists all global labels.
// Output is a table (NAME, COLOR, DESCRIPTION) or JSON when --json is set.
var labelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all labels",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		labels, err := c.ListLabels()
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(labels)
			return nil
		}

		if len(labels) == 0 {
			fmt.Println("No labels found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, tabwriterPadding, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tCOLOR\tDESCRIPTION")
		for _, l := range labels {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", l.Name, l.Color, l.Description)
		}
		return w.Flush()
	},
}

// labelCreateCmd creates a new global label with optional color and description.
// The label name is a required positional argument.
var labelCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a label",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		color, _ := cmd.Flags().GetString("color")
		description, _ := cmd.Flags().GetString("description")

		label, err := c.CreateLabel(args[0], color, description)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(label)
			return nil
		}

		fmt.Printf("Created label: %s\n", label.Name)
		return nil
	},
}

func init() {
	labelCreateCmd.Flags().String("color", "", "Label color (hex, e.g. #ff0000)")
	labelCreateCmd.Flags().String("description", "", "Label description")
}

// labelUpdateCmd updates a label's metadata (color and/or description).
// At least one of --color or --description must be provided.
// Uses cmd.Flags().Changed() to distinguish "not set" from "set to empty".
var labelUpdateCmd = &cobra.Command{
	Use:   "update <name>",
	Short: "Update a label",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		colorChanged := cmd.Flags().Changed("color")
		descChanged := cmd.Flags().Changed("description")

		if !colorChanged && !descChanged {
			return errors.New("at least one of --color or --description is required")
		}

		fields := map[string]string{}
		if colorChanged {
			color, _ := cmd.Flags().GetString("color")
			fields["color"] = color
		}
		if descChanged {
			description, _ := cmd.Flags().GetString("description")
			fields["description"] = description
		}

		c, err := getClient()
		if err != nil {
			return err
		}

		label, err := c.UpdateLabel(args[0], fields)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(label)
			return nil
		}

		fmt.Printf("Updated label: %s\n", label.Name)
		return nil
	},
}

func init() {
	labelUpdateCmd.Flags().String("color", "", "New label color (hex)")
	labelUpdateCmd.Flags().String("description", "", "New label description")
}

// labelDeleteCmd deletes a global label by name.
// Removing a label definition does not affect existing issue-label associations.
var labelDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a label",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		if err := c.DeleteLabel(args[0]); err != nil {
			return err
		}

		if !outputJSON {
			fmt.Printf("Deleted label: %s\n", args[0])
		}
		return nil
	},
}
