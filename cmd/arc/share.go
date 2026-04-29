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
	Kind          string `json:"kind"`
	ID            string `json:"id"`
	AuthorName    string `json:"author_name"`
	CommentType   string `json:"comment_type"`
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
	shareCommentsAccepted bool
	shareCommentsJSON     bool
)

func init() {
	shareCreateCmd.Flags().BoolVar(&shareCreateLocal, "local", false, "Use the local arc-server")
	shareCreateCmd.Flags().BoolVar(&shareCreateRemote, "share", false, "Use the configured remote share server")
	shareCreateCmd.Flags().StringVar(&shareCreateServer, "server", "", "Override server URL")
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
	plain := planPlaintext{
		Version:   1,
		Markdown:  string(md),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
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

// resolveServer returns the server URL and kind ("local" or "shared") based on
// the provided flags.
func resolveServer(local, share bool, override string) (string, string) {
	if override != "" {
		return override, "shared"
	}
	if share {
		if env := os.Getenv("ARC_SHARE_SERVER"); env != "" {
			return env, "shared"
		}
		return "https://share.arc.tools", "shared"
	}
	return cliConfigServerURL(), "local"
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
func printComments(server, id string, key []byte, acceptedOnly, asJSON bool) error {
	plan, events, err := fetchAndDecrypt(server, id, key)
	if err != nil {
		return err
	}
	comments := map[string]commentEvent{}
	resolutions := map[string]resolutionEvent{}
	for _, e := range events {
		var raw json.RawMessage
		if err := paste.DecryptJSON(e.Blob, e.IV, key, &raw); err != nil {
			continue
		}
		var generic struct {
			Kind string `json:"kind"`
		}
		_ = json.Unmarshal(raw, &generic)
		switch generic.Kind {
		case "comment":
			var c commentEvent
			if err := json.Unmarshal(raw, &c); err == nil {
				comments[c.ID] = c
			}
		case "resolution":
			var r resolutionEvent
			if err := json.Unmarshal(raw, &r); err == nil {
				if plan.AuthorName == "" || r.AuthorName == plan.AuthorName {
					resolutions[r.CommentID] = r
				}
			}
		}
	}
	for cid, c := range comments {
		res, ok := resolutions[cid]
		status := "open"
		if ok {
			status = res.Status
		}
		if acceptedOnly && status != "accepted" {
			continue
		}
		if asJSON {
			payload := map[string]any{"comment": c, "status": status}
			if ok && res.Reply != "" {
				payload["reply"] = res.Reply
			}
			out, _ := json.Marshal(payload)
			fmt.Println(string(out))
		} else {
			fmt.Printf("[%s] %s (%s): %s\n", status, c.AuthorName, c.CommentType, c.Body)
		}
	}
	return nil
}

// resolveShareRef parses a share reference which may be either a full share
// URL (e.g. https://share.arc.tools/share/abc12345#k=KEY) or a bare share ID
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
