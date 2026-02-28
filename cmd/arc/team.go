package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/spf13/cobra"
)

// TeamContext holds issues grouped by teammate role for team coordination.
type TeamContext struct {
	Workspace  string               `json:"workspace"`
	Epic       *TeamContextEpic     `json:"epic,omitempty"`
	Roles      map[string]*TeamRole `json:"roles"`
	Unassigned []TeamContextIssue   `json:"unassigned"`
}

// TeamContextEpic describes the parent epic for a team context query.
type TeamContextEpic struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Plan  string `json:"plan,omitempty"`
}

// TeamRole groups issues assigned to a specific teammate role.
type TeamRole struct {
	Issues []TeamContextIssue `json:"issues"`
}

// TeamContextIssue is a compact issue representation for team context output.
type TeamContextIssue struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Priority int      `json:"priority"`
	Status   string   `json:"status"`
	Type     string   `json:"type"`
	Plan     string   `json:"plan,omitempty"`
	Deps     []string `json:"deps,omitempty"`
}

// teamCmd is the parent command for team operations.
var teamCmd = &cobra.Command{
	Use:   "team",
	Short: "Agent team operations",
}

// teamContextCmd outputs team context grouped by teammate roles.
var teamContextCmd = &cobra.Command{
	Use:   "context [epic-id]",
	Short: "Output team context grouped by teammate roles",
	Long: `Output issues grouped by their teammate:* labels.

If an epic ID is given, only children of that epic are included.
Otherwise, all issues with teammate:* labels in the workspace are shown.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		wsID, err := getWorkspaceID()
		if err != nil {
			return err
		}

		var epicID string
		if len(args) > 0 {
			epicID = args[0]
		}

		ctx, err := buildTeamContext(c, wsID, epicID)
		if err != nil {
			return err
		}

		if outputJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(ctx)
		}

		return printTeamContext(ctx)
	},
}

func init() {
	rootCmd.AddCommand(teamCmd)
	teamCmd.AddCommand(teamContextCmd)
}

// buildTeamContext assembles the team context from API calls.
func buildTeamContext(c *client.Client, wsID, epicID string) (*TeamContext, error) {
	tc := &TeamContext{
		Workspace: wsID,
		Roles:     make(map[string]*TeamRole),
	}

	var issues []*types.Issue

	if epicID != "" {
		// Fetch epic details
		epic, err := c.GetIssue(wsID, epicID)
		if err != nil {
			return nil, fmt.Errorf("fetch epic: %w", err)
		}

		tc.Epic = &TeamContextEpic{
			ID:    epic.ID,
			Title: epic.Title,
		}

		// Get epic's plan
		pc, err := c.GetPlanContext(wsID, epicID)
		if err == nil && pc.InlinePlan != nil {
			tc.Epic.Plan = pc.InlinePlan.Text
		}

		// Find children via dependencies endpoint
		children, err := getEpicChildren(c, wsID, epicID)
		if err != nil {
			return nil, fmt.Errorf("fetch children: %w", err)
		}
		issues = children
	} else {
		// Fetch all open issues in the workspace
		allIssues, err := c.ListIssues(wsID, client.ListIssuesOptions{
			Status: "open",
			Limit:  200,
		})
		if err != nil {
			return nil, fmt.Errorf("list issues: %w", err)
		}
		// Also include in_progress issues
		inProgress, err := c.ListIssues(wsID, client.ListIssuesOptions{
			Status: "in_progress",
			Limit:  200,
		})
		if err != nil {
			return nil, fmt.Errorf("list in-progress issues: %w", err)
		}
		issues = append(allIssues, inProgress...)
	}

	// For each issue, check labels and group by teammate:* label.
	// The Issue struct includes Labels when returned from the list API.
	for _, issue := range issues {
		tci := TeamContextIssue{
			ID:       issue.ID,
			Title:    issue.Title,
			Priority: issue.Priority,
			Status:   string(issue.Status),
			Type:     string(issue.IssueType),
		}

		// Check for plan
		pc, err := c.GetPlanContext(wsID, issue.ID)
		if err == nil && pc.InlinePlan != nil {
			tci.Plan = pc.InlinePlan.Text
		}

		// Find teammate:* label
		role := ""
		for _, l := range issue.Labels {
			if strings.HasPrefix(l, "teammate:") {
				role = strings.TrimPrefix(l, "teammate:")
				break
			}
		}

		if role == "" {
			// If no epic filter, skip issues without teammate labels
			if epicID == "" {
				continue
			}
			tc.Unassigned = append(tc.Unassigned, tci)
		} else {
			if tc.Roles[role] == nil {
				tc.Roles[role] = &TeamRole{}
			}
			tc.Roles[role].Issues = append(tc.Roles[role].Issues, tci)
		}
	}

	return tc, nil
}

// getEpicChildren fetches child issues of an epic via the dependencies endpoint.
func getEpicChildren(c *client.Client, wsID, epicID string) ([]*types.Issue, error) {
	// Get dependents (issues that depend on the epic)
	details, err := c.GetIssueDetails(wsID, epicID)
	if err != nil {
		return nil, err
	}

	var children []*types.Issue
	for _, dep := range details.Dependents {
		if dep.Type == types.DepParentChild {
			child, err := c.GetIssue(wsID, dep.IssueID)
			if err != nil {
				continue // Skip issues we can't fetch
			}
			children = append(children, child)
		}
	}

	return children, nil
}

// printTeamContext outputs a human-readable team context table.
func printTeamContext(tc *TeamContext) error {
	if tc.Epic != nil {
		fmt.Printf("Epic: %s - %s\n", tc.Epic.ID, tc.Epic.Title)
		if tc.Epic.Plan != "" {
			fmt.Printf("Plan: %s\n", truncate(tc.Epic.Plan, 80))
		}
		fmt.Println()
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, tabwriterPadding, ' ', 0)
	_, _ = fmt.Fprintln(w, "ROLE\tISSUES\tIDS")
	_, _ = fmt.Fprintln(w, "────\t──────\t───")

	for role, r := range tc.Roles {
		ids := make([]string, 0, len(r.Issues))
		for _, issue := range r.Issues {
			ids = append(ids, issue.ID)
		}
		_, _ = fmt.Fprintf(w, "%s\t%d\t%s\n", role, len(r.Issues), strings.Join(ids, ", "))
	}

	if len(tc.Unassigned) > 0 {
		ids := make([]string, 0, len(tc.Unassigned))
		for _, issue := range tc.Unassigned {
			ids = append(ids, issue.ID)
		}
		_, _ = fmt.Fprintf(w, "unassigned\t%d\t%s\n", len(tc.Unassigned), strings.Join(ids, ", "))
	}

	return w.Flush()
}

// truncate returns s trimmed to maxLen characters with "..." appended if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
