package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// planCmd is the parent command for plan management.
var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Manage plans",
	Long: `Manage plans for issues. Arc supports three plan patterns:

1. Inline Plans - Comment-based plans on individual issues
   arc plan set <issue-id> "plan text"
   arc plan show <issue-id>

2. Parent Epic Pattern - Use parent issue with plan, children inherit
   arc plan set <parent-id> "master plan"
   arc dep add <child-id> <parent-id> --type=parent-child

3. Shared Plans - Standalone plans linkable to multiple issues
   arc plan create "Initiative name"
   arc plan link <plan-id> <issue-1> <issue-2>`,
}

func init() {
	rootCmd.AddCommand(planCmd)

	// Inline plan subcommands
	planCmd.AddCommand(planSetCmd)
	planCmd.AddCommand(planShowCmd)
	planCmd.AddCommand(planHistoryCmd)

	// Shared plan subcommands
	planCmd.AddCommand(planCreateCmd)
	planCmd.AddCommand(planEditCmd)
	planCmd.AddCommand(planListCmd)
	planCmd.AddCommand(planDeleteCmd)
	planCmd.AddCommand(planLinkCmd)
	planCmd.AddCommand(planUnlinkCmd)
}

// ============ Inline Plan Commands ============

var planSetCmd = &cobra.Command{
	Use:   "set <issue-id> [plan text]",
	Short: "Set or update an inline plan on an issue",
	Long: `Set or update an inline plan directly on an issue.
If plan text is provided, sets it directly.
If --editor is used, opens $EDITOR to compose the plan.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		issueID := args[0]
		var text string

		useEditor, _ := cmd.Flags().GetBool("editor")
		if useEditor {
			// Get existing plan as starting content
			existing, _ := c.GetPlanContext(wsID, issueID)
			initialContent := ""
			if existing != nil && existing.InlinePlan != nil {
				initialContent = existing.InlinePlan.Text
			}

			editedText, err := editInEditor(initialContent)
			if err != nil {
				return fmt.Errorf("editor: %w", err)
			}
			text = editedText
		} else if len(args) == 2 {
			text = args[1]
		} else {
			return fmt.Errorf("provide plan text or use --editor")
		}

		if text == "" {
			return fmt.Errorf("plan text cannot be empty")
		}

		comment, err := c.SetInlinePlan(wsID, issueID, text)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(comment)
			return nil
		}

		fmt.Printf("Plan set on %s\n", issueID)
		return nil
	},
}

func init() {
	planSetCmd.Flags().BoolP("editor", "e", false, "Open $EDITOR to compose plan")
}

var planShowCmd = &cobra.Command{
	Use:   "show <issue-id>",
	Short: "Show all plans for an issue",
	Long: `Show the plan context for an issue, including:
- Inline plan (if set directly on this issue)
- Parent plan (inherited from parent-child dependency)
- Shared plans (linked to this issue)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		pc, err := c.GetPlanContext(wsID, args[0])
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(pc)
			return nil
		}

		hasPlan := false

		if pc.InlinePlan != nil {
			hasPlan = true
			fmt.Printf("== Inline Plan ==\n")
			fmt.Printf("Author: %s\n", pc.InlinePlan.Author)
			fmt.Printf("Updated: %s\n", pc.InlinePlan.CreatedAt.Format("2006-01-02 15:04"))
			fmt.Printf("\n%s\n", pc.InlinePlan.Text)
		}

		if pc.ParentPlan != nil {
			if hasPlan {
				fmt.Println()
			}
			hasPlan = true
			fmt.Printf("== Parent Plan (from %s) ==\n", pc.ParentIssueID)
			fmt.Printf("Author: %s\n", pc.ParentPlan.Author)
			fmt.Printf("Updated: %s\n", pc.ParentPlan.CreatedAt.Format("2006-01-02 15:04"))
			fmt.Printf("\n%s\n", pc.ParentPlan.Text)
		}

		if len(pc.SharedPlans) > 0 {
			if hasPlan {
				fmt.Println()
			}
			hasPlan = true
			fmt.Printf("== Shared Plans ==\n")
			for i, plan := range pc.SharedPlans {
				if i > 0 {
					fmt.Println()
				}
				fmt.Printf("[%s] %s\n", plan.ID, plan.Title)
				fmt.Printf("Updated: %s\n", plan.UpdatedAt.Format("2006-01-02 15:04"))
				if plan.Content != "" {
					fmt.Printf("\n%s\n", plan.Content)
				}
			}
		}

		if !hasPlan {
			fmt.Println("No plans found for this issue")
		}

		return nil
	},
}

var planHistoryCmd = &cobra.Command{
	Use:   "history <issue-id>",
	Short: "Show plan version history for an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		history, err := c.GetPlanHistory(wsID, args[0])
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(history)
			return nil
		}

		if len(history) == 0 {
			fmt.Println("No plan history for this issue")
			return nil
		}

		for i, comment := range history {
			if i > 0 {
				fmt.Println("\n---")
			}
			fmt.Printf("Version %d (%s by %s):\n",
				len(history)-i, comment.CreatedAt.Format("2006-01-02 15:04"), comment.Author)
			fmt.Println(comment.Text)
		}

		return nil
	},
}

// ============ Shared Plan Commands ============

var planCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new shared plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		title := args[0]
		content := ""

		useEditor, _ := cmd.Flags().GetBool("editor")
		if useEditor {
			editedContent, err := editInEditor("")
			if err != nil {
				return fmt.Errorf("editor: %w", err)
			}
			content = editedContent
		}

		plan, err := c.CreatePlan(wsID, title, content)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(plan)
			return nil
		}

		fmt.Printf("Created plan: %s\n", plan.ID)
		return nil
	},
}

func init() {
	planCreateCmd.Flags().BoolP("editor", "e", false, "Open $EDITOR to compose content")
}

var planEditCmd = &cobra.Command{
	Use:   "edit <plan-id>",
	Short: "Edit a shared plan in $EDITOR",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		planID := args[0]

		// Get current plan
		plan, err := c.GetPlan(wsID, planID)
		if err != nil {
			return err
		}

		// Edit in $EDITOR
		edited, err := editInEditor(plan.Content)
		if err != nil {
			return fmt.Errorf("editor: %w", err)
		}

		// Update
		updated, err := c.UpdatePlan(wsID, planID, plan.Title, edited)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(updated)
			return nil
		}

		fmt.Printf("Updated plan: %s\n", planID)
		return nil
	},
}

var planListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all shared plans in the workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		plans, err := c.ListPlans(wsID)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(plans)
			return nil
		}

		if len(plans) == 0 {
			fmt.Println("No shared plans found")
			return nil
		}

		for _, plan := range plans {
			linkedCount := len(plan.LinkedIssues)
			fmt.Printf("%s - %s (%d linked)\n", plan.ID, plan.Title, linkedCount)
		}

		return nil
	},
}

var planDeleteCmd = &cobra.Command{
	Use:   "delete <plan-id>",
	Short: "Delete a shared plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		if err := c.DeletePlan(wsID, args[0]); err != nil {
			return err
		}

		fmt.Printf("Deleted plan: %s\n", args[0])
		return nil
	},
}

var planLinkCmd = &cobra.Command{
	Use:   "link <plan-id> <issue>...",
	Short: "Link a plan to one or more issues",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		planID := args[0]
		issueIDs := args[1:]

		if err := c.LinkIssuesToPlan(wsID, planID, issueIDs); err != nil {
			return err
		}

		fmt.Printf("Linked %s to %d issue(s)\n", planID, len(issueIDs))
		return nil
	},
}

var planUnlinkCmd = &cobra.Command{
	Use:   "unlink <plan-id> <issue>...",
	Short: "Unlink a plan from one or more issues",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		planID := args[0]
		issueIDs := args[1:]

		for _, issueID := range issueIDs {
			if err := c.UnlinkIssueFromPlan(wsID, planID, issueID); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to unlink %s: %v\n", issueID, err)
				continue
			}
		}

		fmt.Printf("Unlinked %s from %d issue(s)\n", planID, len(issueIDs))
		return nil
	},
}

// ============ Editor Helper ============

// editInEditor opens the user's $EDITOR with the given content and returns the edited result.
func editInEditor(content string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", "arc-plan-*.md")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())

	// Write initial content
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return "", err
	}
	tmpFile.Close()

	// Open editor
	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	// Read result
	result, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(result)), nil
}
