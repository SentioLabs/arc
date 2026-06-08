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
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sentiolabs/arc/internal/plans"
	"github.com/spf13/cobra"
)

// titleFlag is the --title flag for planCreateCmd, overriding the derived plan title.
var titleFlag string

// noFrontmatter is the --no-frontmatter flag for planCreateCmd, skipping frontmatter on create.
var noFrontmatter bool

// datePrefixRe matches a leading YYYY-MM-DD- date prefix on filenames.
var datePrefixRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-`)

// deriveTitle returns the title for a plan file. It reads the file and returns
// the text of the first line beginning with exactly `# ` (single `#` + space;
// `##` lines are intentionally excluded). If no matching heading is found, it
// falls back to the filename base with any leading YYYY-MM-DD- prefix and
// trailing .md extension removed.
func deriveTitle(path string) string {
	f, err := os.Open(path)
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "# ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "# "))
			}
		}
		if err := scanner.Err(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "warning: could not read %s for title: %v\n", path, err)
		}
	}
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".md")
	base = datePrefixRe.ReplaceAllString(base, "")
	return base
}

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

	planCreateCmd.Flags().StringVar(&titleFlag, "title", "", "Override the plan title written to frontmatter")
	planCreateCmd.Flags().BoolVar(&noFrontmatter, "no-frontmatter", false,
		"Skip writing frontmatter into the plan file")
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

		filePath, err := filepath.Abs(args[0])
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}

		plan, err := c.CreatePlan(filePath)
		if err != nil {
			return err
		}

		if !noFrontmatter {
			title := titleFlag
			if title == "" {
				title = deriveTitle(filePath)
			}
			projName := ""
			if wsID, _, _, e := resolveProject(); e == nil {
				if pr, e2 := c.GetProject(wsID); e2 == nil {
					projName = pr.Name
				}
			}
			meta := plans.Frontmatter{
				Title:     title,
				Date:      time.Now().Format("2006-01-02"),
				Project:   projName,
				Status:    "in_review",
				Tags:      []string{"arc", "design-spec"},
				ArcReview: plans.ArcReview{Kind: "legacy", ID: plan.ID},
			}
			if e := plans.EnsureFrontmatter(filePath, meta); e != nil {
				_, _ = fmt.Fprintf(os.Stderr, "warning: could not write frontmatter: %v\n", e)
			}
		}

		if outputJSON {
			outputResult(plan)
			return nil
		}

		_, _ = fmt.Printf("Plan created: %s (file: %s, status: %s)\n", plan.ID, plan.FilePath, plan.Status)
		_, _ = fmt.Printf("Review at: %s/planner/%s\n", c.BaseURL(), plan.ID)
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

		if p, e := c.GetPlan(planID); e == nil && p.FilePath != "" {
			if e2 := plans.SetStatus(p.FilePath, "approved"); e2 != nil && !errors.Is(e2, plans.ErrNoFrontmatter) {
				_, _ = fmt.Fprintf(os.Stderr, "warning: could not sync status in %s: %v\n", p.FilePath, e2)
			}
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
