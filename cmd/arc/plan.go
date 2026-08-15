// Package main provides the plan management commands for the arc CLI.
// Plans are ephemeral review artifacts backed by filesystem markdown files.
//
// Commands:
//   - plan create: register a new plan from a file path
//   - plan show: display plan metadata and content
//   - plan approve: mark a plan as approved
//   - plan reject: mark a plan as rejected
//   - plan comments: list review comments for a plan
//   - plan wait: block until a review decision is made in the web UI
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
	"github.com/sentiolabs/arc/internal/types"
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

// planWaitTimeout is the --timeout flag for planWaitCmd.
var planWaitTimeout time.Duration

const (
	// planWaitDefaultTimeout is the default value for planWaitCmd's --timeout flag.
	planWaitDefaultTimeout = 30 * time.Minute
	// planWaitPollInterval is how often planWaitCmd polls the plan status.
	planWaitPollInterval = 2 * time.Second
	// quotedTextMaxRunes is the max length of a quoted anchor excerpt before truncation.
	quotedTextMaxRunes = 60
)

// init registers all plan subcommands under the root planCmd.
func init() {
	rootCmd.AddCommand(planCmd)

	planCmd.AddCommand(planCreateCmd)
	planCmd.AddCommand(planShowCmd)
	planCmd.AddCommand(planApproveCmd)
	planCmd.AddCommand(planRejectCmd)
	planCmd.AddCommand(planCommentsCmd)
	planCmd.AddCommand(planWaitCmd)

	planCreateCmd.Flags().StringVar(&titleFlag, "title", "", "Override the plan title written to frontmatter")
	planCreateCmd.Flags().BoolVar(&noFrontmatter, "no-frontmatter", false,
		"Skip writing frontmatter into the plan file")
	planWaitCmd.Flags().DurationVar(&planWaitTimeout, "timeout", planWaitDefaultTimeout,
		"maximum time to wait for a decision")
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

		printPlanComments(comments)
		return nil
	},
}

// printPlanComments prints a plan's review comments in the CLI's human-readable
// format. The legacy `[L<n>] content` and `[overall] content` shapes MUST
// remain byte-identical for backward compatibility. Anchored comments use the
// `[L<n>-L<n>] "quote…" content` shape, and resolved comments (of any shape)
// get a `✓ ` prefix.
func printPlanComments(comments []*types.PlanComment) {
	if len(comments) == 0 {
		fmt.Println("No comments")
		return
	}

	for _, comment := range comments {
		prefix := ""
		if comment.ResolvedAt != nil {
			prefix = "✓ "
		}
		switch {
		case comment.Anchor != nil:
			a := comment.Anchor
			loc := fmt.Sprintf("L%d", a.LineStart)
			if a.LineEnd > a.LineStart {
				loc = fmt.Sprintf("L%d-L%d", a.LineStart, a.LineEnd)
			}
			fmt.Printf("%s[%s] %q %s\n", prefix, loc, truncateQuote(a.QuotedText, quotedTextMaxRunes), comment.Content)
		case comment.LineNumber != nil:
			fmt.Printf("%s[L%d] %s\n", prefix, *comment.LineNumber, comment.Content)
		default:
			fmt.Printf("%s[overall] %s\n", prefix, comment.Content)
		}
	}
}

// truncateQuote collapses whitespace/newlines and truncates to maxRunes with an ellipsis.
func truncateQuote(s string, maxRunes int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}

// planWaitResult is the --json shape returned by planWaitCmd once a decision is made.
type planWaitResult struct {
	Status   string               `json:"status"`
	Comments []*types.PlanComment `json:"comments"`
}

// planWaitCmd blocks until a review decision is made in the planner web UI.
// It polls the plan status every 2s until it leaves draft/in_review, then
// prints the decision and the full comment thread (the same formats used by
// `arc plan comments`). Exits non-zero on timeout so callers can distinguish
// "no decision yet" from a decision.
var planWaitCmd = &cobra.Command{
	Use:   "wait <plan-id>",
	Short: "Block until the plan is approved/rejected/changes-requested in the web UI",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		planID := args[0]

		deadline := time.Now().Add(planWaitTimeout)
		for {
			plan, err := c.GetPlan(planID)
			if err != nil {
				return err
			}
			if plan.Status != types.PlanStatusDraft && plan.Status != types.PlanStatusInReview {
				comments, err := c.ListPlanComments(planID)
				if err != nil {
					return err
				}
				if outputJSON {
					outputResult(planWaitResult{Status: plan.Status, Comments: comments})
					return nil
				}
				fmt.Printf("Decision: %s\n", plan.Status)
				printPlanComments(comments)
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("timed out after %s waiting for a decision on %s (status: %s)",
					planWaitTimeout, planID, plan.Status)
			}
			time.Sleep(planWaitPollInterval)
		}
	},
}
