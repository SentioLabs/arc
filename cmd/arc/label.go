package main

import (
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// labelCmd is the parent command for label management.
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
		fmt.Fprintln(w, "NAME\tCOLOR\tDESCRIPTION")
		for _, l := range labels {
			fmt.Fprintf(w, "%s\t%s\t%s\n", l.Name, l.Color, l.Description)
		}
		return w.Flush()
	},
}

// labelCreateCmd creates a new global label.
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

// labelUpdateCmd updates a label's metadata.
var labelUpdateCmd = &cobra.Command{
	Use:   "update <name>",
	Short: "Update a label",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		color, _ := cmd.Flags().GetString("color")
		description, _ := cmd.Flags().GetString("description")

		if color == "" && description == "" {
			return errors.New("at least one of --color or --description is required")
		}

		c, err := getClient()
		if err != nil {
			return err
		}

		label, err := c.UpdateLabel(args[0], color, description)
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

// labelDeleteCmd deletes a global label.
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
