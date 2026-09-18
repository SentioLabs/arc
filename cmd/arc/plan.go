// Durable plan commands upload and consume exact project-owned revisions.
//
// The CLI has three independent kinds of local artifacts:
//   - drafts, whose bytes are read once for an explicit upload;
//   - exports, which copy one retained revision;
//   - context files, which record the versions a worker or reviewer used.
//
// None of these artifacts is a server storage path. A successful upload may
// enrich its local draft with provenance, but edits afterward require a save.
// Server-owned title, lifecycle, and review state are read through the API.
// Frontmatter carried inside Markdown remains ordinary retained content.
//
// Editing and reviewing use different concurrency counters. Save names its
// expected head. A decision names head, review, and feedback versions. Archive
// and restore use the metadata version. These values are never inferred during
// a mutation; a stale capture is an error the caller must explicitly reconcile.
//
// Create, save, and adoption have durable request keys. The transport preserves
// one key and payload across an interrupted response, and callers can provide
// that key again after process interruption. Other writes are attempted once.
//
// Work context is captured from a transactional claim response. It includes
// every higher-level governing source, not merely the nearest tactical plan.
// Evidence and completion load the original artifact instead of recapturing
// current state. Explicit unlinked state detects a first attachment too.
//
// Export and context publication complete a temporary file before making its
// destination visible. No-clobber publication treats symlinks as existing entries.
// Forced export replaces the destination entry without following its target.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/plans"
	"github.com/sentiolabs/arc/internal/project"
	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/spf13/cobra"
)

var datePrefixRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-`)

const (
	planUpdateName               = "update"
	planHistoryName              = "history"
	planArchived                 = "archived"
	planDispositionsName         = "dispositions"
	planWaitName                 = "wait"
	planSuperseded               = "superseded"
	planCommentsName             = "comments"
	planCommentName              = "comment"
	planCreateName               = "create"
	planFeedbackVersionFlag      = "expected-feedback-version"
	planWaitDefaultTimeout       = 30 * time.Minute
	planWaitPollInterval         = 2 * time.Second
	planWaitMaxConsecutiveErrors = 5
	quotedTextMaxRunes           = 60
	planPageLimit                = 50
)

// deriveTitle reads the first top-level heading, then falls back to the filename.
// A title is client metadata and never selects a server publication path.
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

// printPlanComments retains the existing terminal rendering for comment excerpts.
// Anchors describe original revision text rather than positions in a newer head.
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
			fmt.Printf(
				"%s[%s] %q %s\n",
				prefix,
				loc,
				truncateQuote(a.QuotedText, quotedTextMaxRunes),
				comment.Content,
			)
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

var (
	planCmd           = newPlanCommand()
	planWaitCmd, _, _ = planCmd.Find([]string{planWaitName})
)

func init() { rootCmd.AddCommand(planCmd) }

// Each construction owns its flags so command invocations cannot leak captures.
func newPlanCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "plan", Short: "Upload, review, and retain immutable plan revisions"}
	cmd.AddCommand(
		newPlanCreateCommand(),
		newPlanShowCommand(),
		newPlanUpdateCommand(),
		newPlanExportCommand(),
		newPlanWaitCommand(),
		newPlanAdoptCommand(),
	)
	for _, status := range []struct{ name, status string }{
		{"submit", types.PlanStatusInReview},
		{"approve", types.PlanStatusApproved},
		{"reject", types.PlanStatusRejected},
	} {
		cmd.AddCommand(newPlanDecisionCommand(status.name, status.status))
	}
	for _, item := range []struct{ name, lifecycle string }{{"archive", planArchived}, {"restore", "active"}} {
		cmd.AddCommand(newPlanLifecycleCommand(item.name, item.lifecycle))
	}
	for _, name := range []string{cmdList, planHistoryName, planCommentsName, planDispositionsName} {
		cmd.AddCommand(newPlanListCommand(name))
	}
	cmd.AddCommand(newPlanCommentCommand(), newPlanFeedbackCommand())
	resolve := &cobra.Command{
		Use:   "resolve ISSUE",
		Short: "Resolve the complete pinned design chain",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, p, err := planClient()
			if err != nil {
				return err
			}
			result, err := c.ResolveGoverningPlan(p, args[0])
			if err != nil {
				return err
			}
			outputResult(result)
			return nil
		},
	}
	cmd.AddCommand(resolve)
	return cmd
}

// planClient requires project resolution before any plan request.
// The client configuration does not expose or depend on the server content root.
func planClient() (*client.Client, string, error) {
	c, err := getClient()
	if err != nil {
		return nil, "", err
	}
	p, _, _, err := resolveProject()
	return c, p, err
}

// planWaitClient resolves the same explicit, server-path, and legacy sources
// under the wait deadline. Legacy validation is read-only: waiting never needs
// to register workspace paths or remove local configuration to observe a review.
func planWaitClient(ctx context.Context) (*client.Client, string, error) {
	c, err := getClient()
	if err != nil {
		return nil, "", err
	}
	if projectID != "" {
		return c, projectID, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}
	paths := []string{cwd}
	if canonical := project.NormalizePath(cwd); canonical != cwd {
		paths = append(paths, canonical)
	}
	for _, path := range paths {
		resolved, resolveErr := c.ResolveProjectByPathContext(ctx, path)
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		if resolveErr == nil && resolved.ProjectID != "" {
			return c, resolved.ProjectID, nil
		}
	}
	cfg, err := readLegacyConfig(project.DefaultArcHome(), cwd)
	if err != nil {
		return nil, "", err
	}
	if cfg == nil || cfg.WorkspaceID == "" {
		return nil, "", errors.New("no project configured for this directory; use --project or arc init")
	}
	_, err = c.GetProjectContext(ctx, cfg.WorkspaceID)
	if err != nil {
		return nil, "", err
	}
	return c, cfg.WorkspaceID, nil
}

// stringFlag reads a command-owned string option.
// Presence-sensitive callers separately inspect Changed before using its value.
func stringFlag(cmd *cobra.Command, name string) string {
	value, _ := cmd.Flags().GetString(name)
	return value
}

// boolFlag reads a command-owned switch.
// This helper does not infer a version or select a revision.
func boolFlag(cmd *cobra.Command, name string) bool {
	value, _ := cmd.Flags().GetBool(name)
	return value
}

// int64Flag reads an explicitly named numeric capture.
// Zero is valid for review counters, so callers validate field presence separately.
func int64Flag(cmd *cobra.Command, name string) int64 {
	value, _ := cmd.Flags().GetInt64(name)
	return value
}

// requirePlanFlags distinguishes omitted captures from explicit zero values.
// It runs before mutations rather than consulting current server versions.
func requirePlanFlags(cmd *cobra.Command, names ...string) error {
	for _, name := range names {
		if !cmd.Flags().Changed(name) {
			return fmt.Errorf("--%s is required; use the version captured when work started", name)
		}
	}
	return nil
}

// revisionFlag names the exact content under review or execution.
// Only the convenience show command permits an omitted revision.
func revisionFlag(cmd *cobra.Command) {
	cmd.Flags().Int64("revision", 0, "Exact retained revision (required for workflow commands)")
}

// keyFlag lets interrupted processes replay the same logical write.
// The payload and key must both match the original attempted request.
func keyFlag(cmd *cobra.Command) {
	cmd.Flags().
		String("idempotency-key", "", "Reuse this key and identical input to retry an interrupted upload/save/adoption")
}

// readPlanInput loads the local file before connecting to the server.
// Reject malformed UTF-8 before JSON encoding could replace its bytes.
func readPlanInput(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(b) {
		return "", errors.New("plan content must be valid UTF-8")
	}
	return string(b), nil
}

// newPlanCreateCommand separates upload from optional local metadata enrichment.
// An uploaded source is a draft, with no ongoing synchronization relationship.
func newPlanCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create FILE",
		Short: "Upload local Markdown as immutable revision 1",
		Args:  cobra.ExactArgs(1),
		RunE:  runPlanCreate,
	}
	cmd.Flags().String("title", "", "Override the derived title")
	cmd.Flags().Bool("no-frontmatter", false, "Keep the local file unchanged after upload")
	keyFlag(cmd)
	return cmd
}

// runPlanCreate uploads the exact bytes read before any frontmatter changes.
// A local metadata failure reports the created identity and never repeats creation.
func runPlanCreate(cmd *cobra.Command, args []string) error {
	content, err := readPlanInput(args[0])
	if err != nil {
		return err
	}
	c, p, err := planClient()
	if err != nil {
		return err
	}
	title := stringFlag(cmd, "title")
	if title == "" {
		title = deriveTitle(args[0])
	}
	result, err := c.CreatePlan(
		p,
		stringFlag(cmd, "idempotency-key"),
		storage.PlanUpload{Title: title, Content: content, SourceName: filepath.Base(args[0])},
	)
	if err != nil {
		return err
	}
	if !boolFlag(cmd, "no-frontmatter") {
		meta := planFrontmatter(c, result.Plan, result.Revision, "durable")
		if err := plans.EnsureFrontmatter(args[0], meta); err != nil {
			_, _ = fmt.Fprintf(
				cmd.ErrOrStderr(),
				"warning: created %s revision %d in project %s; could not write local frontmatter: %v\n",
				result.Plan.ID,
				result.Revision.Revision,
				p,
				err,
			)
		}
	}
	if outputJSON {
		outputResult(result)
	} else {
		fmt.Printf("Plan created: %s revision %d (project %s)\n", result.Plan.ID, result.Revision.Revision, p)
		fmt.Printf("Review at: %s/%s/plans/%s/%d\n", c.BaseURL(), p, result.Plan.ID, result.Revision.Revision)
	}
	return nil
}

// planFrontmatter records durable identity and the displayed revision status.
// These local properties are provenance, not authority for future server reviews.
func planFrontmatter(
	c *client.Client,
	plan types.Plan,
	revision types.PlanRevisionWithContent,
	kind string,
) plans.Frontmatter {
	return plans.Frontmatter{
		Title:   plan.Title,
		Status:  revision.ReviewStatus,
		Project: plan.ProjectID,
		Tags:    []string{"arc", "design-spec"},
		ArcReview: plans.ArcReview{
			Kind:      kind,
			ID:        plan.ID,
			ProjectID: plan.ProjectID,
			Revision:  revision.Revision,
			Server:    c.BaseURL(),
		},
	}
}

// Review captures bind both target identity and all server-provided versions.
type planReviewContext struct {
	ProjectID string `json:"project_id"`
	PlanID    string `json:"plan_id"`
	Revision  int64  `json:"revision"`
	types.PlanReviewRequest
}

// newPlanShowCommand offers head convenience while always displaying its number.
// An optional capture saves the review preconditions observed with that read.
func newPlanShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show PLAN",
		Short: "Read a retained revision (defaults to displayed head)",
		Args:  cobra.ExactArgs(1),
		RunE:  runPlanShow,
	}
	revisionFlag(cmd)
	cmd.Flags().
		String("review-context-output", "", "Capture review identity and versions into a JSON file")
	return cmd
}

// runPlanShow pairs metadata with one explicit immutable content read.
// A concurrent save makes its captured review stale rather than changing its target.
func runPlanShow(cmd *cobra.Command, args []string) error {
	c, p, err := planClient()
	if err != nil {
		return err
	}
	meta, err := c.GetPlan(p, args[0])
	if err != nil {
		return err
	}
	n := int64Flag(cmd, "revision")
	if !cmd.Flags().Changed("revision") {
		n = meta.HeadRevision
	}
	revision, err := c.ReadPlanRevision(p, args[0], n)
	if err != nil {
		return err
	}
	if path := stringFlag(cmd, "review-context-output"); path != "" {
		capture := planReviewContext{
			ProjectID: p,
			PlanID:    args[0],
			Revision:  n,
			PlanReviewRequest: types.PlanReviewRequest{
				ExpectedHead:            meta.HeadRevision,
				ExpectedReviewVersion:   revision.ReviewVersion,
				ExpectedFeedbackVersion: meta.FeedbackVersion,
			},
		}
		if err := writeContextJSON(path, capture); err != nil {
			return err
		}
	}
	if outputJSON {
		outputResult(struct {
			Plan     *types.Plan                    `json:"plan"`
			Revision *types.PlanRevisionWithContent `json:"revision"`
		}{meta, revision})
	} else {
		fmt.Printf("Plan: %s — %s\nRevision: %d (head %d)\n", meta.ID, meta.Title, n, meta.HeadRevision)
		fmt.Printf("Review: %s (version %d, feedback %d)\n",
			revision.ReviewStatus, revision.ReviewVersion, meta.FeedbackVersion,
		)
		fmt.Printf("Lifecycle: %s (version %d)\n\n%s", meta.Lifecycle, meta.Version, revision.Content)
	}
	return nil
}

// newPlanUpdateCommand requires the editing base and uploads local content.
// It returns the exact server result, including a replay marker on keyed retries.
func newPlanUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update PLAN FILE",
		Short: "Upload a new revision against a captured head",
		Args:  cobra.ExactArgs(depPairArgCount),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requirePlanFlags(cmd, "expected-revision"); err != nil {
				return err
			}
			content, err := readPlanInput(args[1])
			if err != nil {
				return err
			}
			c, p, err := planClient()
			if err != nil {
				return err
			}
			result, err := c.SavePlanRevision(
				p,
				args[0],
				stringFlag(cmd, "idempotency-key"),
				storage.PlanSave{
					Content:          content,
					SourceName:       filepath.Base(args[1]),
					ExpectedRevision: int64Flag(cmd, "expected-revision"),
				},
			)
			if err != nil {
				return err
			}
			outputResult(result)
			return nil
		},
	}
	cmd.Flags().Int64("expected-revision", 0, "Head revision used when editing")
	keyFlag(cmd)
	return cmd
}

// newPlanDecisionCommand shares the captured concurrency contract across decisions.
// Approval and rejection operate on server content without writing local files.
func newPlanDecisionCommand(name, status string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name + " PLAN",
		Short: "Record a review transition against captured versions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requirePlanFlags(cmd, "revision"); err != nil {
				return err
			}
			c, p, err := planClient()
			if err != nil {
				return err
			}
			req, err := readPlanReviewContext(cmd, p, args[0])
			if err != nil {
				return err
			}
			req.Status = status
			result, err := c.DecidePlanRevision(p, args[0], int64Flag(cmd, "revision"), req)
			if err != nil {
				return err
			}
			outputResult(result)
			return nil
		},
	}
	revisionFlag(cmd)
	for _, name := range []string{"expected-head", "expected-review-version", planFeedbackVersionFlag} {
		cmd.Flags().Int64(name, 0, "Version captured at review start")
	}
	cmd.Flags().String("review-context", "", "Previously captured review JSON from plan show")
	return cmd
}

// readPlanReviewContext accepts either all explicit versions or one stored capture.
// Mixing sources could produce a review contract that was never actually observed.
func readPlanReviewContext(
	cmd *cobra.Command,
	projectID, planID string,
) (types.PlanReviewRequest, error) {
	var capture planReviewContext
	path := stringFlag(cmd, "review-context")
	if path == "" {
		err := requirePlanFlags(
			cmd,
			"expected-head",
			"expected-review-version",
			planFeedbackVersionFlag,
		)
		return types.PlanReviewRequest{
			ExpectedHead:            int64Flag(cmd, "expected-head"),
			ExpectedReviewVersion:   int64Flag(cmd, "expected-review-version"),
			ExpectedFeedbackVersion: int64Flag(cmd, planFeedbackVersionFlag),
		}, err
	}
	for _, name := range []string{"expected-head", "expected-review-version", planFeedbackVersionFlag} {
		if cmd.Flags().Changed(name) {
			return capture.PlanReviewRequest, errors.New(
				"review-context cannot be combined with explicit review versions",
			)
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return capture.PlanReviewRequest, err
	}
	if err := requiredJSONFields(b,
		"project_id", "plan_id", "revision", "expected_head", "expected_review_version", "expected_feedback_version",
	); err != nil {
		return capture.PlanReviewRequest, err
	}
	if err := json.Unmarshal(b, &capture); err != nil {
		return capture.PlanReviewRequest, err
	}
	if capture.ProjectID != projectID || capture.PlanID != planID ||
		capture.Revision != int64Flag(cmd, "revision") {
		return capture.PlanReviewRequest, errors.New(
			"review context identity does not match project, plan, and revision",
		)
	}
	return capture.PlanReviewRequest, nil
}

// newPlanLifecycleCommand applies a reversible metadata transition.
// The metadata version is distinct from content, review, and feedback versions.
func newPlanLifecycleCommand(name, lifecycle string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name + " PLAN",
		Short: "Change lifecycle using the captured metadata version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requirePlanFlags(cmd, "expected-version"); err != nil {
				return err
			}
			c, p, err := planClient()
			if err != nil {
				return err
			}
			result, err := c.UpdatePlan(
				p,
				args[0],
				storage.PlanMetadataUpdate{
					ExpectedVersion: int64Flag(cmd, "expected-version"),
					Lifecycle:       lifecycle,
				},
			)
			if err != nil {
				return err
			}
			outputResult(result)
			return nil
		},
	}
	cmd.Flags().Int64("expected-version", 0, "Captured plan metadata version")
	return cmd
}

// newPlanListCommand bounds every retained collection request.
// Callers can page through archived plans, revisions, feedback, and dispositions.
func newPlanListCommand(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name + " PLAN",
		Short: "List retained " + name,
		Args:  cobra.ExactArgs(1),
		RunE:  runPlanList,
	}
	if name == cmdList {
		cmd.Use = cmdList
		cmd.Args = cobra.NoArgs
		cmd.Flags().Bool(planArchived, false, "List archived plans")
	}
	if name == planCommentsName || name == planDispositionsName {
		revisionFlag(cmd)
	}
	if name == planCommentsName {
		cmd.Flags().
			Bool("include-prior", true, "Include outstanding earlier feedback with original revision anchors")
	}
	cmd.Flags().Int("limit", planPageLimit, "Page size (1–200)")
	cmd.Flags().Int("offset", 0, "Page offset")
	return cmd
}

// runPlanList preserves each server result and original comment provenance.
// Earlier feedback is opt-out, so a reviewer sees outstanding historical discussion.
func runPlanList(cmd *cobra.Command, args []string) error {
	c, p, err := planClient()
	if err != nil {
		return err
	}
	limit, _ := cmd.Flags().GetInt("limit")
	offset, _ := cmd.Flags().GetInt("offset")
	var result any
	switch cmd.Name() {
	case cmdList:
		result, err = c.ListPlans(p, boolFlag(cmd, planArchived), limit, offset)
	case planHistoryName:
		result, err = c.ListPlanRevisions(p, args[0], limit, offset)
	case planCommentsName, planDispositionsName:
		if e := requirePlanFlags(cmd, "revision"); e != nil {
			return e
		}
		if cmd.Name() == planCommentsName {
			result, err = c.ListPlanComments(
				p,
				args[0],
				int64Flag(cmd, "revision"),
				boolFlag(cmd, "include-prior"),
				limit,
				offset,
			)
		} else {
			result, err = c.ListPlanDispositions(p, args[0], int64Flag(cmd, "revision"), limit, offset)
		}
	}
	if err != nil {
		return err
	}
	outputResult(result)
	return nil
}

// newPlanCommentCommand exposes creation and version-checked feedback changes.
// Deletion creates an audit tombstone; disposition remains a separate action.
func newPlanCommentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   planCommentName,
		Short: "Create, edit, reopen, or tombstone revision feedback",
	}
	for _, name := range []string{planCreateName, planUpdateName, "delete"} {
		child := &cobra.Command{
			Use:   name + " PLAN [COMMENT]",
			Short: name + " revision feedback",
			Args:  cobra.ExactArgs(depPairArgCount),
			RunE:  runPlanComment,
		}
		if name == planCreateName {
			child.Use = "create PLAN"
			child.Args = cobra.ExactArgs(1)
		}
		revisionFlag(child)
		child.Flags().String("content", "", "Feedback text")
		child.Flags().String("anchor", "", "JSON file with the original comment anchor")
		child.Flags().Int("line", 0, "Original line number")
		child.Flags().Bool("reopen", false, "Reopen feedback")
		child.Flags().
			Int64("expected-comment-version", 0, "Captured comment version for update/delete")
		cmd.AddCommand(child)
	}
	return cmd
}

// runPlanComment sends anchors and edits to their original revision.
// It never resolves feedback by setting a generic flag without a reason.
func runPlanComment(cmd *cobra.Command, args []string) error {
	if err := requirePlanFlags(cmd, "revision"); err != nil {
		return err
	}
	c, p, err := planClient()
	if err != nil {
		return err
	}
	n := int64Flag(cmd, "revision")
	var anchor *types.PlanCommentAnchor
	if path := stringFlag(cmd, "anchor"); path != "" {
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		if e = json.Unmarshal(b, &anchor); e != nil {
			return e
		}
	}
	var result *types.PlanComment
	if cmd.Name() == planCreateName {
		if err := requirePlanFlags(cmd, "content"); err != nil {
			return err
		}
		req := storage.PlanCommentCreate{Content: stringFlag(cmd, "content"), Anchor: anchor}
		if cmd.Flags().Changed("line") {
			line, _ := cmd.Flags().GetInt("line")
			req.LineNumber = &line
		}
		result, err = c.CreatePlanComment(p, args[0], n, req)
		if err != nil {
			return err
		}
		outputResult(result)
		return nil
	}
	{
		if err := requirePlanFlags(cmd, "expected-comment-version"); err != nil {
			return err
		}
		req := storage.PlanCommentUpdate{
			ExpectedVersion: int64Flag(cmd, "expected-comment-version"),
			Anchor:          anchor,
			Reopen:          boolFlag(cmd, "reopen"),
		}
		if cmd.Flags().Changed("content") {
			content := stringFlag(cmd, "content")
			req.Content = &content
		}
		if cmd.Name() == "delete" {
			result, err = c.DeletePlanComment(p, args[0], n, args[1], req.ExpectedVersion)
		} else {
			result, err = c.UpdatePlanComment(p, args[0], n, args[1], req)
		}
	}
	if err != nil {
		return err
	}
	outputResult(result)
	return nil
}

// newPlanFeedbackCommand binds a reason to a target and exact comment version.
// Its feedback version detects edits made since the reviewer inspected discussion.
func newPlanFeedbackCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feedback PLAN",
		Short: "Record addressed or deferred feedback with a reason",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requirePlanFlags(cmd,
				"revision", planCommentName, "expected-comment-version",
				planFeedbackVersionFlag, "disposition", "reason",
			); err != nil {
				return err
			}
			c, p, err := planClient()
			if err != nil {
				return err
			}
			req := storage.PlanDispositionRequest{
				CommentID:               stringFlag(cmd, planCommentName),
				ExpectedCommentVersion:  int64Flag(cmd, "expected-comment-version"),
				ExpectedFeedbackVersion: int64Flag(cmd, planFeedbackVersionFlag),
				Disposition:             stringFlag(cmd, "disposition"),
				Reason:                  stringFlag(cmd, "reason"),
			}
			result, err := c.AddPlanDisposition(p, args[0], int64Flag(cmd, "revision"), req)
			if err != nil {
				return err
			}
			outputResult(result)
			return nil
		},
	}
	revisionFlag(cmd)
	for _, name := range []string{planCommentName, "disposition", "reason"} {
		cmd.Flags().String(name, "", "Feedback "+name)
	}
	for _, name := range []string{"expected-comment-version", planFeedbackVersionFlag} {
		cmd.Flags().Int64(name, 0, "Captured version")
	}
	return cmd
}

// newPlanExportCommand defaults to byte-identical retained content.
// Metadata enrichment is an explicit alternative and is labelled in both outputs.
func newPlanExportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export PLAN",
		Short: "Atomically export exact retained bytes",
		Args:  cobra.ExactArgs(1),
		RunE:  runPlanExport,
	}
	revisionFlag(cmd)
	cmd.Flags().String("output", "", "Destination file")
	cmd.Flags().Bool("force", false, "Atomically replace an existing destination")
	cmd.Flags().
		Bool("with-frontmatter", false, "Produce a labelled metadata-enriched export (not byte-identical evidence)")
	return cmd
}

// runPlanExport reads server content directly and writes only the chosen destination.
// Metadata enrichment happens in memory before the final atomic filesystem operation.
func runPlanExport(cmd *cobra.Command, args []string) error {
	if err := requirePlanFlags(cmd, "revision", "output"); err != nil {
		return err
	}
	c, p, err := planClient()
	if err != nil {
		return err
	}
	revision, err := c.ReadPlanRevision(p, args[0], int64Flag(cmd, "revision"))
	if err != nil {
		return err
	}
	content := []byte(revision.Content)
	if boolFlag(cmd, "with-frontmatter") {
		meta, e := c.GetPlan(p, args[0])
		if e != nil {
			return e
		}
		content, e = plans.WithFrontmatter(
			content,
			planFrontmatter(c, *meta, *revision, "durable-export"),
		)
		if e != nil {
			return e
		}
	}
	if err := writePlanExport(stringFlag(cmd, "output"), content, boolFlag(cmd, "force")); err != nil {
		return err
	}
	if boolFlag(cmd, "with-frontmatter") {
		fmt.Println(
			"Exported with metadata enrichment; this is not byte-identical revision evidence",
		)
	}
	return nil
}

// Linking a fully written temporary file makes no-clobber atomic, including
// dangling symlinks. Forced rename replaces the directory entry, never its target.
func writePlanExport(path string, content []byte, force bool) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".arc-export-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.Write(content); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if force {
		return os.Rename(name, path)
	}
	if err = os.Link(name, path); err != nil {
		return fmt.Errorf(
			"export destination exists or cannot be created (use --force to replace): %w",
			err,
		)
	}
	return nil
}

type planWaitResult struct {
	Status       string               `json:"status"`
	Revision     int64                `json:"revision"`
	HeadRevision int64                `json:"head_revision"`
	Comments     []*types.PlanComment `json:"comments"`
}

// newPlanWaitCommand watches one captured revision for a bounded duration.
// Timeout and cancellation do not record or modify any review decision.
func newPlanWaitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wait PLAN",
		Short: "Wait for one revision; report a new head as superseded",
		Args:  cobra.ExactArgs(1),
		RunE:  runPlanWait,
	}
	revisionFlag(cmd)
	cmd.Flags().Duration("timeout", planWaitDefaultTimeout, "Maximum wait duration")
	return cmd
}

// runPlanWait retains the requested revision across transient read errors.
// A newer undecided head is reported as supersession rather than followed silently.
func runPlanWait(cmd *cobra.Command, args []string) error {
	if err := cmdContext(cmd).Err(); err != nil {
		return planWaitCancelled(args[0], err)
	}
	if err := requirePlanFlags(cmd, "revision"); err != nil {
		return err
	}
	timeout, _ := cmd.Flags().GetDuration("timeout")
	ctx, cancel := context.WithTimeout(cmdContext(cmd), timeout)
	defer cancel()
	c, p, err := planWaitClient(ctx)
	if ctx.Err() != nil {
		return planWaitCancelled(args[0], ctx.Err())
	}
	if err != nil {
		return err
	}
	n := int64Flag(cmd, "revision")
	failures := 0
	for {
		result, err := pollPlanWaitContext(ctx, c, p, args[0], n)
		if ctx.Err() != nil {
			return planWaitCancelled(args[0], ctx.Err())
		}
		if err != nil {
			failures++
			if failures >= planWaitMaxConsecutiveErrors {
				return err
			}
		} else {
			failures = 0
		}
		if err == nil && result.Status != "" {
			outputResult(result)
			if result.Status == planSuperseded {
				return fmt.Errorf("revision %d superseded by head %d", n, result.HeadRevision)
			}
			return nil
		}
		if !sleepOrCancel(ctx, planWaitPollInterval) {
			return planWaitCancelled(args[0], ctx.Err())
		}
	}
}

// pollPlanWait gives historical decisions precedence over derived supersession.
// All reads remain scoped to the same project, plan, and requested revision.
func pollPlanWait(c *client.Client, p, id string, n int64) (planWaitResult, error) {
	return pollPlanWaitContext(context.Background(), c, p, id, n)
}

// pollPlanWaitContext keeps every content, metadata, and feedback read bounded.
func pollPlanWaitContext(
	ctx context.Context,
	c *client.Client,
	p, id string,
	n int64,
) (planWaitResult, error) {
	result := planWaitResult{Revision: n}
	rev, err := c.ReadPlanRevisionContext(ctx, p, id, n)
	if err != nil {
		return result, err
	}
	// Decided historical outcomes win even when metadata has subsequently advanced.
	switch rev.ReviewStatus {
	case types.PlanStatusApproved, types.PlanStatusRejected:
		result.Status = rev.ReviewStatus
		result.Comments, err = c.ListPlanCommentsContext(ctx, p, id, n, true, planPageLimit, 0)
		return result, err
	}
	meta, err := c.GetPlanContext(ctx, p, id)
	if err != nil {
		return result, err
	}
	result.HeadRevision = meta.HeadRevision
	if meta.HeadRevision != n {
		// A decision may have committed between the first revision read and
		// the metadata read, followed by a save. Re-read that same revision.
		latest, readErr := c.ReadPlanRevisionContext(ctx, p, id, n)
		if readErr != nil {
			return result, readErr
		}
		switch latest.ReviewStatus {
		case types.PlanStatusApproved, types.PlanStatusRejected:
			result.Status = latest.ReviewStatus
			result.Comments, err = c.ListPlanCommentsContext(ctx, p, id, n, true, planPageLimit, 0)
			return result, err
		}
		result.Status = planSuperseded
	} else if rev.ReviewStatus == types.PlanStatusChangesRequested {
		result.Status = rev.ReviewStatus
		result.Comments, err = c.ListPlanCommentsContext(ctx, p, id, n, true, planPageLimit, 0)
	}
	return result, err
}

// planWaitCancelled distinguishes a timeout from external interruption.
// Both remain errors because neither proves a completed review.
func planWaitCancelled(id string, cause error) error {
	if errors.Is(cause, context.DeadlineExceeded) {
		return fmt.Errorf("timed out waiting for a decision on %s: %w", id, cause)
	}
	return fmt.Errorf("stopped waiting for a decision on %s: %w", id, cause)
}

// newPlanAdoptCommand previews reconciliation before the caller supplies reasons.
// A dash selects an explicit detach, using the same capture and transaction rules.
func newPlanAdoptCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "adopt CONTAINER PLAN",
		Short: "Preview or atomically apply a captured reconciliation (PLAN '-' detaches)",
		Args:  cobra.ExactArgs(depPairArgCount),
		RunE:  runPlanAdopt,
	}
	revisionFlag(cmd)
	keyFlag(cmd)
	cmd.Flags().Bool("dry-run", false, "Validate and capture proposal without writing")
	cmd.Flags().
		String("reconciliation", "", "Captured PlanAdoptionRequest JSON with complete dispositions")
	return cmd
}

// runPlanAdopt reads fresh state only when preparing a new dry-run proposal.
// Applying a reviewed manifest preserves every captured version and nullable pin.
func runPlanAdopt(cmd *cobra.Command, args []string) error {
	if args[1] != "-" {
		if err := requirePlanFlags(cmd, "revision"); err != nil {
			return err
		}
	}
	c, p, err := planClient()
	if err != nil {
		return err
	}
	var req types.PlanAdoptionRequest
	var target *types.PlanReference
	if args[1] != "-" {
		target = &types.PlanReference{PlanID: args[1], Revision: int64Flag(cmd, "revision")}
	}
	path := stringFlag(cmd, "reconciliation")
	if path != "" {
		req, err = readAdoptionManifest(path, target)
		if err != nil {
			return err
		}
	} else {
		if !boolFlag(cmd, "dry-run") {
			return errors.New("--reconciliation is required when applying adoption")
		}
		req, err = prepareAdoptionPreview(c, p, args[0], target)
		if err != nil {
			return err
		}
	}
	req.DryRun = boolFlag(cmd, "dry-run")
	result, err := c.AdoptPlan(p, args[0], stringFlag(cmd, "idempotency-key"), req)
	if err != nil {
		return err
	}
	outputResult(result)
	return nil
}

// Presence is validated before decoding so absent expectations cannot turn into
// explicit unlinked expectations through struct zero values.
func validateReconciliationJSON(b []byte) error {
	if err := requiredJSONFields(b, "expected_container_version", "expected_governance_generation"); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	for _, key := range []string{"expected_pin", "target_pin"} {
		if _, ok := raw[key]; !ok {
			return fmt.Errorf("%s is required (explicit null is allowed)", key)
		}
	}
	var tasks []map[string]json.RawMessage
	if len(raw["tasks"]) > 0 {
		if err := json.Unmarshal(raw["tasks"], &tasks); err != nil {
			return err
		}
	}
	for _, task := range tasks {
		if _, err := decodeWorkContext(task["expected"]); err != nil {
			return err
		}
	}
	var pins []map[string]json.RawMessage
	if len(raw["container_pins"]) > 0 {
		if err := json.Unmarshal(raw["container_pins"], &pins); err != nil {
			return err
		}
	}
	for _, pin := range pins {
		for _, key := range []string{"expected_pin", "target_pin", "expected_container_version"} {
			if _, ok := pin[key]; !ok {
				return fmt.Errorf("container pin %s is required", key)
			}
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	var req types.PlanAdoptionRequest
	return decoder.Decode(&req)
}

// readAdoptionManifest binds a caller-reviewed proposal to the requested target.
// Its expected versions remain exactly those captured in the manifest.
func readAdoptionManifest(
	path string,
	target *types.PlanReference,
) (types.PlanAdoptionRequest, error) {
	var req types.PlanAdoptionRequest
	b, err := os.ReadFile(path)
	if err != nil {
		return req, err
	}
	if err = validateReconciliationJSON(b); err != nil {
		return req, err
	}
	if err = json.Unmarshal(b, &req); err != nil {
		return req, err
	}
	if (target == nil) != (req.TargetPin == nil) || (target != nil && *target != *req.TargetPin) {
		return req, errors.New("reconciliation target does not match requested plan and revision")
	}
	return req, nil
}

// prepareAdoptionPreview captures current metadata only for a new dry run.
// Applying a manifest never invokes this helper or refreshes its expectations.
func prepareAdoptionPreview(
	c *client.Client,
	p, containerID string,
	target *types.PlanReference,
) (types.PlanAdoptionRequest, error) {
	var req types.PlanAdoptionRequest
	container, err := c.GetIssueDetails(p, containerID)
	if err != nil {
		return req, err
	}
	project, err := c.GetProject(p)
	if err != nil {
		return req, err
	}
	return types.PlanAdoptionRequest{
		ExpectedContainerVersion:     container.ContractVersion,
		ExpectedGovernanceGeneration: project.GovernanceGeneration,
		ExpectedPin:                  container.GoverningPlan,
		TargetPin:                    target,
	}, nil
}
