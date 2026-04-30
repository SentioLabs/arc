// Package main extends the arc CLI with `arc share` commands for creating
// and managing zero-knowledge encrypted plan shares.
package main

import (
	"encoding/base64"
	"encoding/json"
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
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	CommentID string `json:"comment_id"`
	Status    string `json:"status"`
	Reply     string `json:"reply,omitempty"`
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
	Kind        string  `json:"kind"` // always "edit"
	ID          string  `json:"id"`
	CommentID   string  `json:"comment_id"`
	AuthorName  string  `json:"author_name"`
	Body        *string `json:"body,omitempty"`
	SuggestedText *string `json:"suggested_text,omitempty"`
	CommentType *string `json:"comment_type,omitempty"`
	CreatedAt   string  `json:"created_at"`
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
	Short: "Replace the encrypted plan content (uses edit_token from shares.json)",
	Args:  cobra.ExactArgs(2),
	RunE:  runShareUpdate,
}

var shareDeleteCmd = &cobra.Command{
	Use:   "delete <id-or-url>",
	Short: "Delete a share (uses edit_token from shares.json)",
	Args:  cobra.ExactArgs(1),
	RunE:  runShareDelete,
}

var (
	shareCreateLocal      bool
	shareCreateRemote     bool
	shareCreateServer     string
	shareCreateAuthor     string
	shareCreateTitle      string
	shareCommentsAccepted bool
	shareCommentsJSON     bool
)

func init() {
	shareCreateCmd.Flags().BoolVar(&shareCreateLocal, "local", false, "Use the local arc-server")
	shareCreateCmd.Flags().BoolVar(&shareCreateRemote, "share", false, "Use the configured remote share server")
	shareCreateCmd.Flags().StringVar(&shareCreateServer, "server", "", "Server URL override. With --share, resolution is: --server flag → share_server in ~/.arc/cli-config.json → $ARC_SHARE_SERVER → https://arcplanner.sentiolabs.io.")
	shareCreateCmd.Flags().StringVar(&shareCreateAuthor, "author", "", "Author name embedded in the plan. Resolution: --author flag → share_author in ~/.arc/cli-config.json → $ARC_SHARE_AUTHOR → `git config user.name`. Reviewers who enter this exact name in the share UI gain Accept/Resolve/Reject controls.")
	shareCreateCmd.Flags().StringVar(&shareCreateTitle, "title", "", "Optional plan title shown in the share UI header (defaults to the filename)")
	shareCommentsCmd.Flags().BoolVar(&shareCommentsAccepted, "accepted-only", false, "Only print accepted comments")
	shareCommentsCmd.Flags().BoolVar(&shareCommentsJSON, "json", false, "Output as JSON")

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
	server, kind := resolveServer(shareCreateLocal, shareCreateRemote, shareCreateServer)
	key, err := paste.GenerateKey()
	if err != nil {
		return err
	}
	author := resolveAuthor(shareCreateAuthor)
	if author == "" {
		fmt.Fprintln(os.Stderr, "warning: no author name resolved.")
		fmt.Fprintln(os.Stderr, "         Set one via --author, share_author in ~/.arc/cli-config.json,")
		fmt.Fprintln(os.Stderr, "         $ARC_SHARE_AUTHOR, or `git config user.name`.")
		fmt.Fprintln(os.Stderr, "         Without an author, Accept/Resolve/Reject controls in the share UI")
		fmt.Fprintln(os.Stderr, "         stay hidden for every reviewer.")
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
	fmt.Printf("Share URL:  %s/share/%s#k=%s\n", strings.TrimRight(server, "/"), resp.ID, keyB64)
	fmt.Printf("Edit token: %s (saved in ~/.arc/shares.json — keep safe)\n", resp.EditToken)
	return nil
}

func runShareList(cmd *cobra.Command, args []string) error {
	f, err := sharesconfig.Load()
	if err != nil {
		return err
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
		return fmt.Errorf("no edit_token for share %s in ~/.arc/shares.json", id)
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
		return fmt.Errorf("no edit_token for share %s in ~/.arc/shares.json", id)
	}
	if err := deleteShare(server, id, s.EditToken); err != nil {
		return err
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
// The author name is the only thing that lets the share UI distinguish the
// plan owner from reviewers — when a visitor enters this exact name in the
// SPA's name prompt, they get Accept/Resolve/Reject controls. Without it,
// nobody is recognized as the author and the controls stay hidden for all.
func resolveAuthor(flag string) string {
	if s := strings.TrimSpace(flag); s != "" {
		return s
	}
	if cfg, err := loadConfig(); err == nil {
		if s := strings.TrimSpace(cfg.ShareAuthor); s != "" {
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
// everything in either mode.
func resolveServer(_ bool, share bool, override string) (string, string) {
	if s := strings.TrimSpace(override); s != "" {
		return s, "shared"
	}
	if share {
		return resolveShareServer(), "shared"
	}
	return cliConfigServerURL(), "local"
}

// resolveShareServer returns the URL of the remote paste server, resolving in
// precedence order: config > env > built-in default. (The flag is checked one
// level up in resolveServer.)
func resolveShareServer() string {
	if cfg, err := loadConfig(); err == nil {
		if s := strings.TrimSpace(cfg.ShareServer); s != "" {
			return s
		}
	}
	if s := strings.TrimSpace(os.Getenv("ARC_SHARE_SERVER")); s != "" {
		return s
	}
	return "https://arcplanner.sentiolabs.io"
}

// cliConfigServerURL returns the server URL from the CLI config, falling back
// to the default local URL.
func cliConfigServerURL() string {
	cfg, err := loadConfig()
	if err != nil || cfg.ServerURL == "" {
		return "http://localhost:7432"
	}
	return cfg.ServerURL
}

// postCreate sends a CreatePasteRequest to the server and returns the response.
func postCreate(server string, blob, iv []byte) (*paste.CreatePasteResponse, error) {
	u := strings.TrimRight(server, "/") + "/api/paste"
	body, _ := json.Marshal(paste.CreatePasteRequest{PlanBlob: blob, PlanIV: iv, SchemaVer: 1})
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
func fetchAndDecrypt(server, id string, key []byte) (*planPlaintext, []paste.PasteEvent, error) {
	resp, err := http.Get(strings.TrimRight(server, "/") + "/api/paste/" + url.PathEscape(id))
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("get paste: %s", resp.Status)
	}
	var pr struct {
		paste.PasteShare
		Events []paste.PasteEvent `json:"events"`
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
	type decoded struct {
		kind string
		raw  json.RawMessage
		ts   string
		eid  string
	}
	decodedEvents := make([]decoded, 0, len(events))
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
		decodedEvents = append(decodedEvents, decoded{
			kind: generic.Kind,
			raw:  raw,
			ts:   generic.CreatedAt,
			eid:  generic.ID,
		})
	}
	// Stable chronological order for deterministic edit application.
	sort.SliceStable(decodedEvents, func(i, j int) bool {
		if decodedEvents[i].ts != decodedEvents[j].ts {
			return decodedEvents[i].ts < decodedEvents[j].ts
		}
		return decodedEvents[i].eid < decodedEvents[j].eid
	})

	comments := map[string]commentEvent{}
	resolutions := map[string]resolutionEvent{}
	for _, d := range decodedEvents {
		switch d.kind {
		case "comment":
			var c commentEvent
			if err := json.Unmarshal(d.raw, &c); err == nil {
				comments[c.ID] = c
			}
		case "resolution":
			var r resolutionEvent
			if err := json.Unmarshal(d.raw, &r); err == nil {
				if plan.AuthorName == "" || r.AuthorName == plan.AuthorName {
					resolutions[r.CommentID] = r
				}
			}
		case "edit":
			var ed editEvent
			if err := json.Unmarshal(d.raw, &ed); err != nil {
				continue
			}
			c, ok := comments[ed.CommentID]
			if !ok {
				continue
			}
			// Replay-time auth: edits are accepted from either the comment's
			// original author OR the plan author (so the author can sharpen
			// thin reviewer feedback like "expand this more"). The displayed
			// comment.author_name is unchanged either way — only the edit
			// event itself records who actually edited. Forged events from
			// third parties are silently dropped.
			//
			// `plan.AuthorName` must be non-empty before granting plan-owner
			// edit rights; otherwise empty strings on both sides would all
			// match and accidentally authorize anonymous edits.
			isOriginal := ed.AuthorName == c.AuthorName
			isPlanAuthor := plan.AuthorName != "" && ed.AuthorName == plan.AuthorName
			if !isOriginal && !isPlanAuthor {
				continue
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
	}
	// Build a deterministic-order list of comment entries so multiple runs
	// produce identical output (Go map iteration is randomized).
	entries := make([]commentEntry, 0, len(comments))
	for cid, c := range comments {
		res, ok := resolutions[cid]
		status := "open"
		reply := ""
		if ok {
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

	if asJSON {
		return emitBundle(id, plan, entries)
	}
	for _, e := range entries {
		// Mark deletes visually so they don't get mistaken for empty-body comments.
		prefix := ""
		if e.comment.Action == "delete" {
			prefix = "[delete] "
		}
		fmt.Printf("[%s] %s%s (%s): %s\n", e.status, prefix, e.comment.AuthorName, e.comment.CommentType, e.comment.Body)
	}
	return nil
}

// shareBundle is the JSON shape emitted by `arc share comments --json`.
//
// Designed for LLM consumption: a single object with everything an agent
// needs to apply review feedback.
//
// Plan content is exposed via exactly one of two fields:
//   - `file` — absolute path on disk. Set when the share is registered in
//     shares.json AND the file is readable. Agent reads it directly.
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
	// share is in shares.json and the file is readable.
	File string `json:"file,omitempty"`
	// MarkdownB64 is the plan content, base64-encoded. Present iff File
	// is not set. Decode with standard base64 (RawStdEncoding-compatible).
	MarkdownB64 string `json:"markdown_b64,omitempty"`
}

type bundleComment struct {
	Comment        commentEvent           `json:"comment"`
	Status         string                 `json:"status"`
	Reply          string                 `json:"reply,omitempty"`
	ResolvedAnchor *bundleResolvedAnchor  `json:"resolved_anchor,omitempty"`
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
//   - If the share is in shares.json AND the recorded file is readable,
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
// URL (e.g. https://arcplanner.sentiolabs.io/share/abc12345#k=KEY) or a bare share ID
// known to ~/.arc/shares.json.
func resolveShareRef(ref string) (string, string, []byte, error) {
	if strings.Contains(ref, "://") {
		u, err := url.Parse(ref)
		if err != nil {
			return "", "", nil, err
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 2 || parts[0] != "share" {
			return "", "", nil, fmt.Errorf("invalid share URL: %s", ref)
		}
		id := parts[1]
		frag, _ := url.ParseQuery(u.Fragment)
		keyB64 := frag.Get("k")
		if keyB64 == "" {
			if s, _ := sharesconfig.Find(id); s != nil {
				keyB64 = s.KeyB64Url
			}
		}
		key, err := base64.RawURLEncoding.DecodeString(keyB64)
		if err != nil {
			return "", "", nil, err
		}
		return id, u.Scheme + "://" + u.Host, key, nil
	}
	s, err := sharesconfig.Find(ref)
	if err != nil {
		return "", "", nil, err
	}
	if s == nil {
		return "", "", nil, fmt.Errorf("unknown share id: %s", ref)
	}
	key, _ := base64.RawURLEncoding.DecodeString(s.KeyB64Url)
	return s.ID, s.URL, key, nil
}
