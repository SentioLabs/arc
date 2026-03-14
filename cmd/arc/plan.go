// Package main provides the plan management commands for the arc CLI.
// Plans are ephemeral review artifacts backed by filesystem markdown files.
//
// Commands:
//   - plan create: register a new plan from a file path
//   - plan show: display plan metadata and content
//   - plan approve: mark a plan as approved
//   - plan reject: mark a plan as rejected
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// planCmd is the parent command for plan management.
var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Manage plans",
	Long: `Manage ephemeral plan review artifacts.

Commands:
  create <file-path>       Register a plan from a markdown file
  show <plan-id>           Show plan metadata and content
  approve <plan-id>        Approve a plan
  reject <plan-id>         Reject a plan`,
}

// init registers all plan subcommands under the root planCmd.
func init() {
	rootCmd.AddCommand(planCmd)

	planCmd.AddCommand(planCreateCmd)
	planCmd.AddCommand(planShowCmd)
	planCmd.AddCommand(planApproveCmd)
	planCmd.AddCommand(planRejectCmd)
	planCmd.AddCommand(planCommentsCmd)
}

// planCreateCmd registers a new plan from a file path.
var planCreateCmd = &cobra.Command{
	Use:   "create <file-path>",
	Short: "Register a new plan from a markdown file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		filePath := args[0]

		plan, err := c.CreatePlan(filePath)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(plan)
			return nil
		}

		_, _ = fmt.Printf("Plan created: %s (file: %s, status: %s)\n", plan.ID, plan.FilePath, plan.Status)
		return nil
	},
}

// planShowCmd displays a plan's metadata and content.
var planShowCmd = &cobra.Command{
	Use:   "show <plan-id>",
	Short: "Show plan metadata and content",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		plan, err := c.GetPlan(args[0])
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(plan)
			return nil
		}

		_, _ = fmt.Printf("Plan: %s\n", plan.ID)
		_, _ = fmt.Printf("File: %s\n", plan.FilePath)
		_, _ = fmt.Printf("Status: %s\n", plan.Status)
		_, _ = fmt.Printf("Updated: %s\n", plan.UpdatedAt.Format("2006-01-02 15:04"))
		if plan.Content != "" {
			_, _ = fmt.Printf("\n%s\n", plan.Content)
		}

		return nil
	},
}

// planApproveCmd approves a plan.
var planApproveCmd = &cobra.Command{
	Use:   "approve <plan-id>",
	Short: "Approve a plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		planID := args[0]

		if err := c.UpdatePlanStatus(planID, "approved"); err != nil {
			return err
		}

		_, _ = fmt.Printf("Plan %s approved\n", planID)
		return nil
	},
}

// planRejectCmd rejects a plan.
var planRejectCmd = &cobra.Command{
	Use:   "reject <plan-id>",
	Short: "Reject a plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		planID := args[0]

		if err := c.UpdatePlanStatus(planID, "rejected"); err != nil {
			return err
		}

		_, _ = fmt.Printf("Plan %s rejected\n", planID)
		return nil
	},
}

// planCommentsCmd lists review comments for a plan in a structured format.
var planCommentsCmd = &cobra.Command{
	Use:   "comments <plan-id>",
	Short: "List review comments for a plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		comments, err := c.ListPlanComments(args[0])
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(comments)
			return nil
		}

		if len(comments) == 0 {
			fmt.Println("No comments")
			return nil
		}

		for _, comment := range comments {
			if comment.LineNumber != nil {
				fmt.Printf("[L%d] %s\n", *comment.LineNumber, comment.Content)
			} else {
				fmt.Printf("[overall] %s\n", comment.Content)
			}
		}
		return nil
	},
}

