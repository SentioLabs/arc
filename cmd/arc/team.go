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
	Deps     []string `json:"deps,omitempty"`
}

// teamListLimit is the max number of issues to fetch per API call.
const teamListLimit = 200

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

		wsID, err := getProjectID()
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

	issues, err := fetchTeamIssues(c, wsID, epicID, tc)
	if err != nil {
		return nil, err
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

		// Find teammate:* label
		role := ""
		for _, l := range issue.Labels {
			if after, ok := strings.CutPrefix(l, "teammate:"); ok {
				role = after
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

// fetchTeamIssues fetches issues for team context, either from an epic or from the full project.
// When epicID is provided, it populates tc.Epic and returns the epic's children.
// Otherwise, it returns all open + in_progress issues.
func fetchTeamIssues(c *client.Client, wsID, epicID string, tc *TeamContext) ([]*types.Issue, error) {
	if epicID != "" {
		return fetchEpicChildren(c, wsID, epicID, tc)
	}
	return fetchProjectIssues(c, wsID)
}

// fetchEpicChildren fetches epic details and its child issues.
func fetchEpicChildren(c *client.Client, wsID, epicID string, tc *TeamContext) ([]*types.Issue, error) {
	epic, err := c.GetIssue(wsID, epicID)
	if err != nil {
		return nil, fmt.Errorf("fetch epic: %w", err)
	}

	tc.Epic = &TeamContextEpic{
		ID:    epic.ID,
		Title: epic.Title,
	}

	children, err := c.ListIssues(wsID, client.ListIssuesOptions{
		Parent: epicID,
		Limit:  teamListLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch children: %w", err)
	}
	return children, nil
}

// fetchProjectIssues fetches all open and in_progress issues from the project.
func fetchProjectIssues(c *client.Client, wsID string) ([]*types.Issue, error) {
	allIssues, err := c.ListIssues(wsID, client.ListIssuesOptions{
		Status: "open",
		Limit:  teamListLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}

	inProgress, err := c.ListIssues(wsID, client.ListIssuesOptions{
		Status: "in_progress",
		Limit:  teamListLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("list in-progress issues: %w", err)
	}

	return append(allIssues, inProgress...), nil
}

// printTeamContext outputs a human-readable team context table.
func printTeamContext(tc *TeamContext) error {
	if tc.Epic != nil {
		fmt.Printf("Epic: %s - %s\n", tc.Epic.ID, tc.Epic.Title)
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

