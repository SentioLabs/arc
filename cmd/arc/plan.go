// Package main provides the plan management commands for the arc CLI.
// Plans are attached to individual issues and support draft/approved/rejected workflow.
//
// Commands:
//   - plan set: create or update a plan on an issue (inline text, stdin, or editor)
//   - plan show: display the current plan for an issue
//   - plan list: list all plans in the project with optional status filter
//   - plan approve: mark a plan as approved
//   - plan reject: mark a plan as rejected
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// planSetMinArgs is the minimum number of arguments for the plan set command.
const planSetMinArgs = 2

// planCmd is the parent command for plan management.
var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Manage plans",
	Long: `Manage plans for issues.

Commands:
  set <issue-id> [text]    Set or update a plan on an issue
  show <issue-id>          Show the plan for an issue
  list                     List plans in the project
  approve <plan-id>        Approve a plan
  reject <plan-id>         Reject a plan`,
}

// init registers all plan subcommands under the root planCmd.
func init() {
	rootCmd.AddCommand(planCmd)

	planCmd.AddCommand(planSetCmd)
	planCmd.AddCommand(planShowCmd)
	planCmd.AddCommand(planListCmd)
	planCmd.AddCommand(planApproveCmd)
	planCmd.AddCommand(planRejectCmd)
}

// planSetCmd sets or updates a plan on an issue.
var planSetCmd = &cobra.Command{
	Use:   "set <issue-id> [plan text]",
	Short: "Set or update a plan on an issue",
	Long: `Set or update a plan on an issue.
If plan text is provided, sets it directly.
If --editor is used, opens $EDITOR to compose the plan.
If --stdin is used, reads plan content from stdin.`,
	Args: cobra.RangeArgs(1, planSetMinArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getProjectID()
		if err != nil {
			return err
		}

		issueID := args[0]
		var text string

		useStdin, _ := cmd.Flags().GetBool("stdin")
		if useStdin {
			content, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			text = strings.TrimSpace(string(content))
		}

		useEditor, _ := cmd.Flags().GetBool("editor")
		if !useStdin && useEditor {
			// Get existing plan as starting content
			existing, _ := c.GetPlanByIssue(wsID, issueID)
			initialContent := ""
			if existing != nil {
				initialContent = existing.Content
			}

			editedText, err := editInEditor(initialContent)
			if err != nil {
				return fmt.Errorf("editor: %w", err)
			}
			text = editedText
		} else if !useStdin && len(args) == planSetMinArgs {
			text = args[1]
		} else if !useStdin {
			return errors.New("provide plan text, use --editor, or use --stdin")
		}

		if text == "" {
			return errors.New("plan text cannot be empty")
		}

		status, _ := cmd.Flags().GetString("status")
		if status == "" {
			status = "draft"
		}

		plan, err := c.SetPlan(wsID, issueID, text, status)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(plan)
			return nil
		}

		_, _ = fmt.Printf("Plan saved on %s (status: %s)\n", issueID, plan.Status)
		return nil
	},
}

// init adds flags to planSetCmd.
func init() {
	planSetCmd.Flags().BoolP("editor", "e", false, "Open $EDITOR to compose plan")
	planSetCmd.Flags().Bool("stdin", false, "Read plan content from stdin")
	planSetCmd.Flags().String("status", "", "Plan status (draft, approved)")
}

// planShowCmd displays the plan for an issue.
var planShowCmd = &cobra.Command{
	Use:   "show <issue-id>",
	Short: "Show the plan for an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getProjectID()
		if err != nil {
			return err
		}

		plan, err := c.GetPlanByIssue(wsID, args[0])
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(plan)
			return nil
		}

		_, _ = fmt.Printf("Plan: %s\n", plan.Title)
		_, _ = fmt.Printf("Status: %s\n", plan.Status)
		_, _ = fmt.Printf("Updated: %s\n", plan.UpdatedAt.Format("2006-01-02 15:04"))
		_, _ = fmt.Printf("\n%s\n", plan.Content)

		return nil
	},
}

// planListCmd lists plans in the project.
var planListCmd = &cobra.Command{
	Use:   "list",
	Short: "List plans in the project",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getProjectID()
		if err != nil {
			return err
		}

		status, _ := cmd.Flags().GetString("status")

		plans, err := c.ListPlans(wsID, status)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(plans)
			return nil
		}

		if len(plans) == 0 {
			_, _ = fmt.Println("No plans found")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, tabwriterPadding, ' ', 0)
		_, _ = fmt.Fprintln(w, "ID\tTITLE\tISSUE\tSTATUS\tUPDATED")
		_, _ = fmt.Fprintln(w, "──\t─────\t─────\t──────\t───────")

		for _, plan := range plans {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				plan.ID,
				plan.Title,
				plan.IssueID,
				plan.Status,
				plan.UpdatedAt.Format("2006-01-02 15:04"),
			)
		}

		return w.Flush()
	},
}

// init adds flags to planListCmd.
func init() {
	planListCmd.Flags().String("status", "", "Filter by status (draft, approved, rejected)")
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

		wsID, err := getProjectID()
		if err != nil {
			return err
		}

		planID := args[0]

		if err := c.UpdatePlanStatus(wsID, planID, "approved"); err != nil {
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

		wsID, err := getProjectID()
		if err != nil {
			return err
		}

		planID := args[0]

		if err := c.UpdatePlanStatus(wsID, planID, "rejected"); err != nil {
			return err
		}

		_, _ = fmt.Printf("Plan %s rejected\n", planID)
		return nil
	},
}

// ============ Editor Helper ============

// editInEditor opens the user's $EDITOR with the given content and returns the edited result.
// It creates a temporary file, writes the initial content, launches the editor, and reads
// back the result. Falls back to $VISUAL and then "vi" if $EDITOR is not set.
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
		_ = tmpFile.Close()
		return "", err
	}
	_ = tmpFile.Close()

	// Open editor
	//nolint:gosec // editor is from user's $EDITOR/$VISUAL env; this is intentional
	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	// Read result
	result, err := os.ReadFile(tmpFile.Name()) //nolint:gosec // reading back our own temp file
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(result)), nil
}
