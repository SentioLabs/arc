// Package main extends the arc CLI with `arc share` commands for creating
// and managing zero-knowledge encrypted plan shares.
package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sentiolabs/arc/internal/paste"
	"github.com/sentiolabs/arc/internal/sharesconfig"
)

// --- plaintext schemas (mirror web/src/lib/paste/types.ts) ---

type planPlaintext struct {
	Version    int    `json:"version"`
	Markdown   string `json:"markdown"`
	Title      string `json:"title,omitempty"`
	AuthorName string `json:"author_name,omitempty"`
	CreatedAt  string `json:"created_at"`
}

type commentEvent struct {
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	AuthorName  string `json:"author_name"`
	CommentType string `json:"comment_type"`
	// Action is the reviewer's primary intent: "comment" (default) or
	// "delete" (strikethrough — body may be empty since the strikethrough
	// IS the action). Preserved on round-trip so consumers like
	// `arc share comments --json` can distinguish deletion requests from
	// regular comments. Mirrors the AnnotationAction type in the SPA
	// (web/src/lib/paste/types.ts).
	Action        string `json:"action,omitempty"`
	Severity      string `json:"severity,omitempty"`
	Body          string `json:"body"`
	SuggestedText string `json:"suggested_text,omitempty"`
	ParentID      string `json:"parent_id,omitempty"`
	Anchor        any    `json:"anchor"`
	CreatedAt     string `json:"created_at"`
}

type resolutionEvent struct {
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	CommentID  string `json:"comment_id"`
	Status     string `json:"status"`
	Reply      string `json:"reply,omitempty"`
	AuthorName string `json:"author_name"`
	CreatedAt  string `json:"created_at"`
}

type approvalEvent struct {
	Kind       string `json:"kind"` // always "approval"
	ID         string `json:"id"`
	AuthorName string `json:"author_name"`
	CreatedAt  string `json:"created_at"`
}

// commentEntry is the in-flight aggregation of a comment + its resolution
// status, used internally by printComments / emitBundle. Lives at package
// scope so both functions can refer to the same type.
type commentEntry struct {
	comment commentEvent
	status  string
	reply   string
}

// editEvent is a reviewer revising their own annotation. Replay merges the
// supplied fields onto the target comment in chronological order, gated on
// the edit's author_name matching the original comment's. See the
// EditEvent docstring in web/src/lib/paste/types.ts for the field semantics.
//
// Only `body`, `suggested_text`, and `comment_type` are editable. Pointer
// fields distinguish "field omitted (keep)" from "field set to empty
// (clear)" — Go zero values would conflate the two.
type editEvent struct {
	Kind          string  `json:"kind"` // always "edit"
	ID            string  `json:"id"`
	CommentID     string  `json:"comment_id"`
	AuthorName    string  `json:"author_name"`
	Body          *string `json:"body,omitempty"`
	SuggestedText *string `json:"suggested_text,omitempty"`
	CommentType   *string `json:"comment_type,omitempty"`
	CreatedAt     string  `json:"created_at"`
}

// retractionEvent is the original commenter taking their annotation back.
// Replay marks the target comment with status='retracted'; the printers
// and JSON bundle filter retracted entries out of the default output so
// downstream LLM consumers don't act on revoked material. The encrypted
// event remains in the log as audit. Only an event whose author_name
// matches the target's author_name takes effect — plan author canNOT
// retract someone else's comment (they have Reject for that).
type retractionEvent struct {
	Kind       string `json:"kind"` // always "retraction"
	ID         string `json:"id"`
	CommentID  string `json:"comment_id"`
	AuthorName string `json:"author_name"`
	CreatedAt  string `json:"created_at"`
}

// --- commands ---

var shareCmd = &cobra.Command{
	Use:   "share",
	Short: "Create and manage encrypted plan shares",
}

var shareCreateCmd = &cobra.Command{
	Use:   "create <plan-file>",
	Short: "Encrypt a plan and create a share",
	Args:  cobra.ExactArgs(1),
	RunE:  runShareCreate,
}

var shareListCmd = &cobra.Command{
	Use:   "list",
	Short: "List shares known to this machine",
	RunE:  runShareList,
}

var shareShowCmd = &cobra.Command{
	Use:   "show <id-or-url>",
	Short: "Decrypt and print plan content",
	Args:  cobra.ExactArgs(1),
	RunE:  runShareShow,
}

var shareCommentsCmd = &cobra.Command{
	Use:   "comments <id-or-url>",
	Short: "Fetch and decrypt comments for a share",
	Args:  cobra.ExactArgs(1),
	RunE:  runShareComments,
}

var sharePullCmd = &cobra.Command{
	Use:   "pull <id-or-url>",
	Short: "Pull comments (alias for `comments` with --accepted-only by default)",
	Args:  cobra.ExactArgs(1),
	RunE:  runSharePull,
}

var shareApproveCmd = &cobra.Command{
	Use:   "approve <id-or-url>",
	Short: "Mark the share as approved",
	Args:  cobra.ExactArgs(1),
	RunE:  runShareApprove,
}

var shareUpdateCmd = &cobra.Command{
	Use:   "update <id-or-url> <plan-file>",
	Short: "Replace the encrypted plan content (uses the edit_token from the local arc keyring)",
	// `update` takes exactly the share ref AND the plan file path.
	Args: cobra.ExactArgs(shareUpdateArgCount),
	RunE: runShareUpdate,
}

const shareUpdateArgCount = 2

// shareKindLocal / shareKindShared label the resolved server in the local
// arc keyring and surface in `arc share list` output.
const (
	shareKindLocal  = "local"
	shareKindShared = "shared"
)

const defaultShareServer = "https://arcplanner.sentiolabs.io"

var shareDeleteCmd = &cobra.Command{
	Use:          "delete <id-or-url>",
	Short:        "Delete a share (uses the edit_token from the local arc keyring)",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runShareDelete,
}

var (
	shareCreateRemote     bool
	shareCreateServer     string
	shareCreateAuthor     string
	shareCreateTitle      string
	shareCommentsAccepted bool
	shareCommentsJSON     bool
	shareShowAuthorURL    bool
	shareDeleteForce      bool
)

func init() {
	shareCreateCmd.Flags().BoolVar(&shareCreateRemote, "remote", false,
		"Use the configured remote share server (precedence: --server flag > share_server in "+
			"cli-config.json > $ARC_SHARE_SERVER > built-in default). Without --remote, --server, "+
			"or an explicit URL, the share is created on the local arc-server.")
	shareCreateCmd.Flags().StringVar(&shareCreateServer, "server", "",
		"Server URL override (precedence: flag > share_server in cli-config.json > "+
			"$ARC_SHARE_SERVER > built-in default).")
	shareCreateCmd.Flags().StringVar(&shareCreateAuthor, "author", "",
		"Author name embedded in the plan (precedence: flag > share_author in "+
			"cli-config.json > $ARC_SHARE_AUTHOR > `git config user.name`). "+
			"Auto-populates the name chip when the Author URL is opened and is "+
			"used for display attribution. Author privileges are gated by the "+
			"&t=<edit_token> in the Author URL, not by this name.")
	shareCreateCmd.Flags().StringVar(&shareCreateTitle, "title", "",
		"Optional plan title shown in the share UI header (defaults to the filename)")
	shareCommentsCmd.Flags().BoolVar(&shareCommentsAccepted, "accepted-only", false, "Only print accepted comments")
	shareCommentsCmd.Flags().BoolVar(&shareCommentsJSON, "json", false, "Output as JSON")
	shareShowCmd.Flags().BoolVar(&shareShowAuthorURL, "author-url", false,
		"Print the author URL (includes edit_token) instead of the plan content")
	shareDeleteCmd.Flags().BoolVarP(&shareDeleteForce, "force", "f", false,
		"Remove the local registry entry even if the edit token is missing or the server delete fails")

	shareCmd.AddCommand(shareCreateCmd, shareListCmd, shareShowCmd, shareCommentsCmd,
		sharePullCmd, shareApproveCmd, shareUpdateCmd, shareDeleteCmd)
	rootCmd.AddCommand(shareCmd)
}

// --- run* functions ---

func runShareCreate(cmd *cobra.Command, args []string) error {
	planFile := args[0]
	md, err := os.ReadFile(planFile)
	if err != nil {
		return err
	}
	server, kind := resolveServer(shareCreateRemote, shareCreateServer)
	key, err := paste.GenerateKey()
	if err != nil {
		return err
	}
	author := resolveAuthor(shareCreateAuthor)
	if author == "" {
		_, _ = fmt.Fprintln(os.Stderr, "warning: no author name resolved.")
		_, _ = fmt.Fprintln(os.Stderr, "         Set one via --author, share_author in ~/.arc/cli-config.json,")
		_, _ = fmt.Fprintln(os.Stderr, "         $ARC_SHARE_AUTHOR, or `git config user.name`.")
		_, _ = fmt.Fprintln(os.Stderr, "         Without an author, Accept/Resolve/Reject controls in the share UI")
		_, _ = fmt.Fprintln(os.Stderr, "         stay hidden for every reviewer.")
	}
	title := shareCreateTitle
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(planFile), ".md")
	}
	plain := planPlaintext{
		Version:    1,
		Markdown:   string(md),
		Title:      title,
		AuthorName: author,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	blob, iv, err := paste.EncryptJSON(plain, key)
	if err != nil {
		return err
	}
	resp, err := postCreate(server, blob, iv)
	if err != nil {
		return err
	}
	keyB64 := base64.RawURLEncoding.EncodeToString(key)
	if err := sharesconfig.Add(sharesconfig.Share{
		ID:        resp.ID,
		Kind:      kind,
		URL:       server,
		KeyB64Url: keyB64,
		EditToken: resp.EditToken,
		PlanFile:  planFile,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return err
	}
	trimmedServer := strings.TrimRight(server, "/")
	authorURL := fmt.Sprintf("%s/share/%s#k=%s&t=%s", trimmedServer, resp.ID, keyB64, resp.EditToken)
	// Reviewer URL is intentionally NOT printed — copy-pasting the wrong line
	// would hand a recipient author privileges (the &t=<editToken> grants
	// Accept/Resolve/Reject). Authors get a reviewer URL by opening the link
	// below and clicking the in-page "Share link" button, which strips &t=.
	if kind == shareKindLocal {
		fmt.Printf("Preview URL (local-only — not reachable by others):\n  %s\n\n", authorURL)
	} else {
		fmt.Printf("Author URL (keep private — open it, then use the in-page "+
			"Share link button to copy a reviewer URL):\n  %s\n\n", authorURL)
	}
	fmt.Println("Edit token saved to the local arc keyring")
	return nil
}

// shareListEntry is the shape emitted by `arc share list --json`.
// Edit tokens are deliberately excluded — they're bearer secrets that
// belong in the keyring, not in machine-readable output.
type shareListEntry struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	URL       string    `json:"url"`
	KeyB64Url string    `json:"key_b64url,omitempty"`
	PlanFile  string    `json:"plan_file,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func runShareList(cmd *cobra.Command, args []string) error {
	f, err := sharesconfig.Load()
	if err != nil {
		return err
	}
	if outputJSON {
		entries := make([]shareListEntry, 0, len(f.Shares))
		for _, s := range f.Shares {
			entries = append(entries, shareListEntry{
				ID:        s.ID,
				Kind:      s.Kind,
				URL:       s.URL,
				KeyB64Url: s.KeyB64Url,
				PlanFile:  s.PlanFile,
				CreatedAt: s.CreatedAt,
			})
		}
		outputResult(entries)
		return nil
	}
	if len(f.Shares) == 0 {
		fmt.Println("(no shares)")
		return nil
	}
	for _, s := range f.Shares {
		fmt.Printf("%s\t%s\t%s\t%s\n", s.ID, s.Kind, s.URL, s.PlanFile)
	}
	return nil
}

func runShareShow(cmd *cobra.Command, args []string) error {
	if shareShowAuthorURL {
		return printAuthorURL(args[0])
	}
	id, server, key, err := resolveShareRef(args[0])
	if err != nil {
		return err
	}
	plan, _, err := fetchAndDecrypt(server, id, key)
	if err != nil {
		return err
	}
	fmt.Println(plan.Markdown)
	return nil
}

func printAuthorURL(ref string) error {
	// Resolve to id; accept either bare id or full share URL.
	id, _, _, err := resolveShareRef(ref)
	if err != nil {
		return err
	}
	s, _ := sharesconfig.Find(id)
	if s == nil || s.EditToken == "" || s.KeyB64Url == "" {
		return fmt.Errorf("no edit_token for share %s in the local arc keyring "+
			"(--author-url requires a share registered on this machine)", id)
	}
	fmt.Printf("%s/share/%s#k=%s&t=%s\n",
		strings.TrimRight(s.URL, "/"), id, s.KeyB64Url, s.EditToken)
	return nil
}

func runShareComments(cmd *cobra.Command, args []string) error {
	id, server, key, err := resolveShareRef(args[0])
	if err != nil {
		return err
	}
	return printComments(server, id, key, shareCommentsAccepted, shareCommentsJSON)
}

func runSharePull(cmd *cobra.Command, args []string) error {
	id, server, key, err := resolveShareRef(args[0])
	if err != nil {
		return err
	}
	return printComments(server, id, key, true, false)
}

func runShareApprove(cmd *cobra.Command, args []string) error {
	id, server, key, err := resolveShareRef(args[0])
	if err != nil {
		return err
	}
	plan, _, err := fetchAndDecrypt(server, id, key)
	if err != nil {
		return err
	}
	ev := approvalEvent{
		Kind:       "approval",
		ID:         fmt.Sprintf("a-%d", time.Now().UnixNano()),
		AuthorName: plan.AuthorName,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	blob, iv, err := paste.EncryptJSON(ev, key)
	if err != nil {
		return err
	}
	return postEvent(server, id, blob, iv)
}

func runShareUpdate(cmd *cobra.Command, args []string) error {
	ref, planFile := args[0], args[1]
	md, err := os.ReadFile(planFile)
	if err != nil {
		return err
	}
	id, server, key, err := resolveShareRef(ref)
	if err != nil {
		return err
	}
	s, _ := sharesconfig.Find(id)
	if s == nil || s.EditToken == "" {
		return fmt.Errorf("no edit_token for share %s in the local arc keyring", id)
	}
	plain := planPlaintext{
		Version:   1,
		Markdown:  string(md),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	blob, iv, err := paste.EncryptJSON(plain, key)
	if err != nil {
		return err
	}
	return putPlan(server, id, s.EditToken, blob, iv)
}

func runShareDelete(cmd *cobra.Command, args []string) error {
	id, server, _, err := resolveShareRef(args[0])
	if err != nil {
		return err
	}
	s, _ := sharesconfig.Find(id)
	if s == nil || s.EditToken == "" {
		if !shareDeleteForce {
			return fmt.Errorf("no edit_token for share %s in the local arc keyring "+
				"(use --force to remove the local entry)", id)
		}
		_, _ = fmt.Fprintf(os.Stderr, "warning: no edit_token for %s; skipping server delete\n", id)
	} else if err := deleteShare(server, id, s.EditToken); err != nil {
		if !shareDeleteForce {
			return err
		}
		_, _ = fmt.Fprintf(os.Stderr, "warning: server delete failed: %v; removing local entry anyway\n", err)
	}
	return sharesconfig.Remove(id)
}

// --- helpers ---

// resolveAuthor returns the author name to embed in the plan plaintext.
// Resolution order (highest priority first):
//  1. explicit --author flag
//  2. share_author in ~/.arc/cli-config.json
//  3. $ARC_SHARE_AUTHOR
//  4. `git config user.name`
//
// Returns "" if none of these produce a value.
//
// The author name is used for: (a) auto-populating the name chip when the
// Author URL is opened in the SPA, (b) display attribution on resolution and
// edit events, and (c) the replay-time guard in events.ts that drops forged
// edits/resolutions. Author UI privileges (Accept/Resolve/Reject) are gated
// by the &t=<edit_token> in the Author URL fragment, not by this name.
func resolveAuthor(flag string) string {
	if s := strings.TrimSpace(flag); s != "" {
		return s
	}
	if cfg, err := loadConfig(); err == nil {
		if s := strings.TrimSpace(cfg.Share.Author); s != "" {
			return s
		}
	}
	if s := strings.TrimSpace(os.Getenv("ARC_SHARE_AUTHOR")); s != "" {
		return s
	}
	out, err := exec.Command("git", "config", "user.name").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// resolveServer returns the server URL and kind ("local" or "shared") based
// on the provided flags. For shared (remote) mode, the URL is resolved with
// precedence:
//
//	--server flag > share_server in ~/.arc/cli-config.json > $ARC_SHARE_SERVER > https://arcplanner.sentiolabs.io
//
// For local mode, the server URL comes from `server_url` in the CLI config
// (defaulting to http://localhost:7432). The `--server` flag still wins over
// everything in either mode — passing a URL there forces shared mode regardless
// of `--remote`.
func resolveServer(remote bool, override string) (server, kind string) {
	if s := strings.TrimSpace(override); s != "" {
		return s, shareKindShared
	}
	if remote {
		return resolveShareServer(), shareKindShared
	}
	return cliConfigServerURL(), shareKindLocal
}

// resolveShareServer returns the URL of the remote paste server, resolving in
// precedence order: config > env > built-in default. (The flag is checked one
// level up in resolveServer.)
func resolveShareServer() string {
	if cfg, err := loadConfig(); err == nil {
		// Only treat config as an override when the user has explicitly set a
		// non-default server URL. If it's still the built-in default, fall
		// through to the env-var and built-in tiers so the precedence chain
		// cli-config > $ARC_SHARE_SERVER > built-in is preserved.
		if s := strings.TrimSpace(cfg.Share.Server); s != "" && s != defaultShareServer {
			return s
		}
	}
	if s := strings.TrimSpace(os.Getenv("ARC_SHARE_SERVER")); s != "" {
		return s
	}
	return defaultShareServer
}

// cliConfigServerURL returns the server URL from the CLI config, falling back
// to the default local URL.
func cliConfigServerURL() string {
	cfg, err := loadConfig()
	if err != nil || cfg.CLI.Server == "" {
		return "http://localhost:7432"
	}
	return cfg.CLI.Server
}

// postCreate sends a CreatePasteRequest to the server and returns the response.
func postCreate(server string, blob, iv []byte) (*paste.CreatePasteResponse, error) {
	u := strings.TrimRight(server, "/") + "/api/paste"
	body, _ := json.Marshal(paste.CreatePasteRequest{PlanBlob: blob, PlanIV: iv, SchemaVer: 1})
	// Variable URL is the entire point of the CLI — the user picks the server.
	//nolint:gosec // G107: intentional user-supplied server URL
	resp, err := http.Post(u, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create paste: %s: %s", resp.Status, b)
	}
	var out paste.CreatePasteResponse
	return &out, json.NewDecoder(resp.Body).Decode(&out)
}

// postEvent appends an encrypted event blob to an existing share.
func postEvent(server, id string, blob, iv []byte) error {
	u := strings.TrimRight(server, "/") + "/api/paste/" + url.PathEscape(id) + "/blobs"
	body, _ := json.Marshal(paste.AppendEventRequest{Blob: blob, IV: iv})
	// Variable URL is the entire point of the CLI — the user picks the server.
	//nolint:gosec // G107: intentional user-supplied server URL
	resp, err := http.Post(u, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("append event: %s: %s", resp.Status, b)
	}
	return nil
}

// putPlan replaces the plan blob of an existing share using the edit token.
func putPlan(server, id, token string, blob, iv []byte) error {
	u := strings.TrimRight(server, "/") + "/api/paste/" + url.PathEscape(id)
	body, _ := json.Marshal(map[string][]byte{"plan_blob": blob, "plan_iv": iv})
	req, _ := http.NewRequest(http.MethodPut, u, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update plan: %s: %s", resp.Status, b)
	}
	return nil
}

// deleteShare deletes a share using the edit token.
func deleteShare(server, id, token string) error {
	u := strings.TrimRight(server, "/") + "/api/paste/" + url.PathEscape(id)
	req, _ := http.NewRequest(http.MethodDelete, u, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete share: %s: %s", resp.Status, b)
	}
	return nil
}

// fetchAndDecrypt retrieves a share from the server and decrypts the plan
// blob using the provided key.
func fetchAndDecrypt(server, id string, key []byte) (*planPlaintext, []paste.Event, error) {
	// Variable URL is the entire point of the CLI — the user picks the server.
	getURL := strings.TrimRight(server, "/") + "/api/paste/" + url.PathEscape(id)
	resp, err := http.Get(getURL) //nolint:gosec // G107: intentional user-supplied server URL
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("get paste: %s", resp.Status)
	}
	var pr struct {
		paste.Share
		Events []paste.Event `json:"events"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, nil, err
	}
	var plan planPlaintext
	if err := paste.DecryptJSON(pr.PlanBlob, pr.PlanIV, key, &plan); err != nil {
		return nil, nil, err
	}
	return &plan, pr.Events, nil
}

// printComments fetches all events, decrypts them, and prints comments to
// stdout. When acceptedOnly is true only accepted comments are printed. When
// asJSON is true each comment is printed as a JSON object.
//
// Replay logic mirrors web/src/lib/paste/events.ts:
//   - 'comment' events seed the state map.
//   - 'resolution' events set the status, gated on author_name matching the
//     plan's author (so reviewers can't self-accept).
//   - 'edit' events merge body/suggested_text/comment_type onto the target
//     comment, gated on author_name matching the comment's original author.
//
// Events are ordered by created_at (then by id as a deterministic tiebreaker)
// so the latest edit wins.
func printComments(server, id string, key []byte, acceptedOnly, asJSON bool) error {
	plan, events, err := fetchAndDecrypt(server, id, key)
	if err != nil {
		return err
	}
	decoded := decodeAndSortEvents(events, key)
	comments, resolutions, retracted := replayEvents(decoded, plan.AuthorName)
	entries := buildCommentEntries(comments, resolutions, retracted, acceptedOnly)
	if asJSON {
		return emitBundle(id, plan, entries)
	}
	printCommentEntries(entries)
	return nil
}

// decodedEvent is the intermediate form used to sort events chronologically
// before replaying them. raw is kept around so the typed unmarshal can run in
// the replay loop without re-decrypting.
type decodedEvent struct {
	kind string
	raw  json.RawMessage
	ts   string
	eid  string
}

func decodeAndSortEvents(events []paste.Event, key []byte) []decodedEvent {
	out := make([]decodedEvent, 0, len(events))
	for _, e := range events {
		var raw json.RawMessage
		if err := paste.DecryptJSON(e.Blob, e.IV, key, &raw); err != nil {
			continue
		}
		var generic struct {
			Kind      string `json:"kind"`
			ID        string `json:"id"`
			CreatedAt string `json:"created_at"`
		}
		if err := json.Unmarshal(raw, &generic); err != nil {
			continue
		}
		out = append(out, decodedEvent{kind: generic.Kind, raw: raw, ts: generic.CreatedAt, eid: generic.ID})
	}
	// Stable chronological order for deterministic edit application.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ts != out[j].ts {
			return out[i].ts < out[j].ts
		}
		return out[i].eid < out[j].eid
	})
	return out
}

func replayEvents(
	events []decodedEvent,
	planAuthor string,
) (map[string]commentEvent, map[string]resolutionEvent, map[string]bool) {
	comments := map[string]commentEvent{}
	resolutions := map[string]resolutionEvent{}
	retracted := map[string]bool{}
	for _, d := range events {
		switch d.kind {
		case "comment":
			applyCommentEvent(d.raw, comments)
		case "resolution":
			applyResolutionEvent(d.raw, planAuthor, resolutions)
		case "edit":
			applyEditEvent(d.raw, planAuthor, comments)
		case "retraction":
			applyRetractionEvent(d.raw, comments, retracted)
		}
	}
	return comments, resolutions, retracted
}

// applyRetractionEvent marks the target comment as retracted iff the event's
// author_name matches the comment's original author_name. Plan author canNOT
// retract someone else's comment — they have Reject (with reply) for that
// purpose. Forged retractions are silently dropped, matching the edit-event
// authorization model.
func applyRetractionEvent(raw json.RawMessage, comments map[string]commentEvent, retracted map[string]bool) {
	var r retractionEvent
	if err := json.Unmarshal(raw, &r); err != nil {
		return
	}
	target, ok := comments[r.CommentID]
	if !ok {
		return
	}
	if r.AuthorName != target.AuthorName {
		return
	}
	retracted[r.CommentID] = true
}

func applyCommentEvent(raw json.RawMessage, comments map[string]commentEvent) {
	var c commentEvent
	if err := json.Unmarshal(raw, &c); err == nil {
		comments[c.ID] = c
	}
}

func applyResolutionEvent(raw json.RawMessage, planAuthor string, resolutions map[string]resolutionEvent) {
	var r resolutionEvent
	if err := json.Unmarshal(raw, &r); err != nil {
		return
	}
	if planAuthor == "" || r.AuthorName == planAuthor {
		resolutions[r.CommentID] = r
	}
}

// applyEditEvent merges body/suggested_text/comment_type fields onto the
// target comment. Replay-time auth: edits are accepted from either the
// comment's original author OR the plan author (so the author can sharpen
// thin reviewer feedback like "expand this more"). The displayed
// comment.author_name is unchanged either way — only the edit event itself
// records who actually edited. Forged events from third parties are silently
// dropped. planAuthor must be non-empty before granting plan-owner edit
// rights; otherwise empty strings on both sides would all match and
// accidentally authorize anonymous edits.
func applyEditEvent(raw json.RawMessage, planAuthor string, comments map[string]commentEvent) {
	var ed editEvent
	if err := json.Unmarshal(raw, &ed); err != nil {
		return
	}
	c, ok := comments[ed.CommentID]
	if !ok {
		return
	}
	isOriginal := ed.AuthorName == c.AuthorName
	isPlanAuthor := planAuthor != "" && ed.AuthorName == planAuthor
	if !isOriginal && !isPlanAuthor {
		return
	}
	if ed.Body != nil {
		c.Body = *ed.Body
	}
	if ed.SuggestedText != nil {
		c.SuggestedText = *ed.SuggestedText
	}
	if ed.CommentType != nil {
		c.CommentType = *ed.CommentType
	}
	comments[ed.CommentID] = c
}

func buildCommentEntries(
	comments map[string]commentEvent,
	resolutions map[string]resolutionEvent,
	retracted map[string]bool,
	acceptedOnly bool,
) []commentEntry {
	// Build a deterministic-order list of comment entries so multiple runs
	// produce identical output (Go map iteration is randomized).
	entries := make([]commentEntry, 0, len(comments))
	for cid, c := range comments {
		// Retracted comments are filtered from default output; the encrypted
		// event still lives in the log for audit but downstream LLM consumers
		// shouldn't see (and act on) revoked material.
		if retracted[cid] {
			continue
		}
		status := "open"
		reply := ""
		if res, ok := resolutions[cid]; ok {
			status = res.Status
			reply = res.Reply
		}
		if acceptedOnly && status != "accepted" {
			continue
		}
		entries = append(entries, commentEntry{comment: c, status: status, reply: reply})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].comment.CreatedAt < entries[j].comment.CreatedAt
	})
	return entries
}

func printCommentEntries(entries []commentEntry) {
	for _, e := range entries {
		// Mark deletes visually so they don't get mistaken for empty-body comments.
		prefix := ""
		if e.comment.Action == "delete" {
			prefix = "[delete] "
		}
		fmt.Printf("[%s] %s%s (%s): %s\n",
			e.status, prefix, e.comment.AuthorName, e.comment.CommentType, e.comment.Body)
	}
}

// shareBundle is the JSON shape emitted by `arc share comments --json`.
//
// Designed for LLM consumption: a single object with everything an agent
// needs to apply review feedback.
//
// Plan content is exposed via exactly one of two fields:
//   - `file` — absolute path on disk. Set when the share is registered in
//     the local arc keyring AND the file is readable. Agent reads it directly.
//   - `markdown_b64` — base64-encoded markdown. Set when there's no local
//     file (e.g. an agent consuming a shared URL it didn't create). Base64
//     avoids JSON escape bloat (every \n and \" doubles the byte count and
//     destroys readability) for markdown payloads that can hit tens of KB.
//
// resolved_anchor line numbers are always computed against whichever source
// `file` or `markdown_b64` exposes, so they're ground-truth for the content
// the agent will actually see.
type shareBundle struct {
	Plan     bundlePlan      `json:"plan"`
	Comments []bundleComment `json:"comments"`
}

type bundlePlan struct {
	ID         string `json:"id"`
	Title      string `json:"title,omitempty"`
	AuthorName string `json:"author_name,omitempty"`
	// File is the absolute path the agent should Edit. Present iff the
	// share is in the local arc keyring and the file is readable.
	File string `json:"file,omitempty"`
	// MarkdownB64 is the plan content, base64-encoded. Present iff File
	// is not set. Decode with standard base64 (RawStdEncoding-compatible).
	MarkdownB64 string `json:"markdown_b64,omitempty"`
}

type bundleComment struct {
	Comment        commentEvent          `json:"comment"`
	Status         string                `json:"status"`
	Reply          string                `json:"reply,omitempty"`
	ResolvedAnchor *bundleResolvedAnchor `json:"resolved_anchor,omitempty"`
}

type bundleResolvedAnchor struct {
	Status    string `json:"status"` // "ok" | "drifted" | "orphaned"
	LineStart int    `json:"line_start"`
	LineEnd   int    `json:"line_end"`
	Snippet   string `json:"snippet,omitempty"`
}

// emitBundle assembles and prints the JSON bundle for `--json` output.
//
// Plan content sourcing:
//   - If the share is in the local arc keyring AND the recorded file is readable,
//     emit `file` (absolute path) and run anchor resolution against the
//     file's current bytes. The agent reads the file directly.
//   - Otherwise, emit `markdown_b64` containing the encrypted blob's
//     markdown, base64-encoded to avoid JSON escape noise.
func emitBundle(id string, plan *planPlaintext, entries []commentEntry) error {
	// `markdown` is what we resolve anchors against — the same bytes the
	// agent will operate on, whether that's the local file or the shared
	// blob. We then expose either `file` or `markdown_b64` to the agent
	// based on what they have access to.
	markdown := plan.Markdown
	planFile := ""
	if s, _ := sharesconfig.Find(id); s != nil && s.PlanFile != "" {
		if data, err := os.ReadFile(s.PlanFile); err == nil {
			markdown = string(data)
			abs, absErr := filepath.Abs(s.PlanFile)
			if absErr == nil {
				planFile = abs
			} else {
				planFile = s.PlanFile
			}
		}
	}

	bp := bundlePlan{
		ID:         id,
		Title:      plan.Title,
		AuthorName: plan.AuthorName,
	}
	if planFile != "" {
		bp.File = planFile
	} else {
		bp.MarkdownB64 = base64.StdEncoding.EncodeToString([]byte(markdown))
	}

	bundle := shareBundle{
		Plan:     bp,
		Comments: make([]bundleComment, 0, len(entries)),
	}

	for _, e := range entries {
		bc := bundleComment{Comment: e.comment, Status: e.status, Reply: e.reply}

		// Re-encode the anchor (which arrived as `any`) and decode into the
		// typed struct, so we can run resolution. If the anchor is malformed
		// we leave resolved_anchor unset rather than fail the whole output.
		if e.comment.Anchor != nil {
			if raw, err := json.Marshal(e.comment.Anchor); err == nil {
				var anc paste.Anchor
				if json.Unmarshal(raw, &anc) == nil {
					r := paste.ResolveAnchor(markdown, anc)
					bc.ResolvedAnchor = &bundleResolvedAnchor{
						Status:    r.Status,
						LineStart: r.LineStart,
						LineEnd:   r.LineEnd,
						Snippet:   paste.Snippet(markdown, r),
					}
				}
			}
		}
		bundle.Comments = append(bundle.Comments, bc)
	}

	out, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

// resolveShareRef parses a share reference which may be either a full share
// URL (e.g. https://arcplanner.sentiolabs.io/share/abc12345#k=KEY) or a bare
// share ID known to the local arc keyring.
func resolveShareRef(ref string) (id, server string, key []byte, err error) {
	if strings.Contains(ref, "://") {
		return resolveShareURL(ref)
	}
	s, ferr := sharesconfig.Find(ref)
	if ferr != nil {
		if errors.Is(ferr, sharesconfig.ErrShareNotFound) {
			return "", "", nil, fmt.Errorf("unknown share id: %s", ref)
		}
		return "", "", nil, ferr
	}
	key, _ = base64.RawURLEncoding.DecodeString(s.KeyB64Url)
	return s.ID, s.URL, key, nil
}

// resolveShareURL is the URL branch of resolveShareRef, split out to keep the
// nesting depth low. Falls back to the local arc keyring for the key if the
// URL has no fragment.
func resolveShareURL(ref string) (id, server string, key []byte, err error) {
	u, perr := url.Parse(ref)
	if perr != nil {
		return "", "", nil, perr
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "share" {
		return "", "", nil, fmt.Errorf("invalid share URL: %s", ref)
	}
	id = parts[1]
	frag, _ := url.ParseQuery(u.Fragment)
	keyB64 := frag.Get("k")
	if keyB64 == "" {
		if s, ferr := sharesconfig.Find(id); ferr == nil {
			keyB64 = s.KeyB64Url
		}
	}
	key, derr := base64.RawURLEncoding.DecodeString(keyB64)
	if derr != nil {
		return "", "", nil, derr
	}
	return id, u.Scheme + "://" + u.Host, key, nil
}
