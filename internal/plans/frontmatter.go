// Package plans handles arc-owned design-spec markdown frontmatter.
package plans

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type ArcReview struct {
	Kind string `yaml:"kind"` // always "legacy" going forward
	ID   string `yaml:"id"`
}

type Frontmatter struct {
	Title     string    `yaml:"title"`
	Date      string    `yaml:"date"`
	Project   string    `yaml:"project"`
	Status    string    `yaml:"status"`
	Tags      []string  `yaml:"tags"`
	ArcReview ArcReview `yaml:"arc_review"`
}

var (
	fmDelim     = []byte("---\n")
	fmDelimCRLF = []byte("---\r\n")
)

// ErrNoFrontmatter is returned when a file has no frontmatter or no status line.
var ErrNoFrontmatter = errors.New("no frontmatter status line")

// findClosingDelim locates the first occurrence of a line that is exactly "---"
// (the closing frontmatter delimiter) within b, starting the search after the
// opening "---\n" has already been consumed.
//
// Returns the byte offset of the '\n' that precedes "---", or -1 if not found.
// Handles two forms of the closing delimiter:
//   - "\n---\n"  — standard case (newline after ---)
//   - "\n---"    — EOF case (--- is the very last bytes with no trailing newline)
//
// Lines that start with "---" but have additional characters (e.g. "----", "--- x")
// are NOT treated as closing delimiters.
func findClosingDelim(b []byte) int {
	search := b
	offset := 0
	for {
		idx := bytes.Index(search, []byte("\n---"))
		if idx < 0 {
			return -1
		}
		// Position of the character immediately following "---".
		after := idx + len(fmDelim)
		if after == len(search) {
			// "---" is at the very end of b with no following character — valid EOF closer.
			return offset + idx
		}
		if search[after] == '\n' || search[after] == '\r' {
			// Followed by newline (LF or CR) — exact closer found.
			return offset + idx
		}
		// Not an exact "---" line; skip past this match and keep searching.
		advance := idx + len(fmDelim)
		offset += advance
		search = search[advance:]
	}
}

// readRawFrontmatter splits b into the raw (unparsed) YAML frontmatter bytes
// and the body that follows, without unmarshaling. ok=false if no leading
// --- block is present (legacy doc). The opening delimiter is recognized in
// both LF ("---\n") and CRLF ("---\r\n") forms.
func readRawFrontmatter(b []byte) (fm []byte, body []byte, ok bool) {
	var openLen int
	switch {
	case bytes.HasPrefix(b, fmDelim):
		openLen = len(fmDelim)
	case bytes.HasPrefix(b, fmDelimCRLF):
		openLen = len(fmDelimCRLF)
	default:
		return nil, b, false
	}
	rest := b[openLen:]
	end := findClosingDelim(rest)
	if end < 0 {
		return nil, b, false
	}
	fm = rest[:end]
	// Skip past the closing "---" line (including its trailing newline, if present).
	after := rest[end+1:] // skip the leading '\n', now points at "---..."
	if i := bytes.IndexByte(after, '\n'); i >= 0 {
		body = after[i+1:]
	} else {
		// "---" at EOF with no trailing newline — body is empty (not nil).
		body = []byte{}
	}
	return fm, body, true
}

// ReadFrontmatter parses a leading --- block. ok=false if absent (legacy doc).
func ReadFrontmatter(b []byte) (fm Frontmatter, body []byte, ok bool, err error) {
	raw, body, ok := readRawFrontmatter(b)
	if !ok {
		return Frontmatter{}, body, false, nil
	}
	if err := yaml.Unmarshal(raw, &fm); err != nil {
		return Frontmatter{}, b, false, err
	}
	return fm, body, true, nil
}

// EnsureFrontmatter idempotently merges the arc-owned frontmatter keys into the
// file's existing frontmatter (creating it if absent), preserving all other
// keys and unioning tags. Atomic.
//
// The merge operates on the parsed *yaml.Node tree rather than a map[string]any,
// so untouched user keys keep their original scalar text (bare dates such as
// `created: 2026-01-01` are not reformatted to RFC3339), their comments, their
// key order, and any anchors/aliases — none of which survive a decode into a
// Go map.
func EnsureFrontmatter(path string, meta Frontmatter) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	body := raw
	var doc yaml.Node
	if fm, rest, ok := readRawFrontmatter(raw); ok {
		if uerr := yaml.Unmarshal(fm, &doc); uerr != nil {
			return uerr
		}
		body = rest
	}

	root := rootMappingNode(&doc)
	mergeFrontmatterNode(root, meta)

	y, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	_, _ = buf.Write(fmDelim)
	_, _ = buf.Write(y)
	_, _ = buf.WriteString("---\n")
	_, _ = buf.Write(body)
	return atomicWrite(path, buf.Bytes())
}

// rootMappingNode returns the mapping node holding the frontmatter key/value
// pairs, initializing doc into a well-formed DocumentNode wrapping an empty
// mapping when the frontmatter was absent, empty, or not a mapping.
func rootMappingNode(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 && doc.Content[0].Kind == yaml.MappingNode {
		return doc.Content[0]
	}
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	doc.Kind = yaml.DocumentNode
	doc.Tag = ""
	doc.Content = []*yaml.Node{root}
	return root
}

// scalarNode builds a plain string scalar node.
func scalarNode(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}

// findMapValue returns the value node paired with key in a MappingNode's flat
// [key, val, key, val, ...] Content slice, or nil if the key is absent.
func findMapValue(root *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			return root.Content[i+1]
		}
	}
	return nil
}

// setMapEntry sets or replaces the value node for key, appending a new key/value
// pair when the key is absent. On replace the existing key position is kept, so
// user key order is preserved.
func setMapEntry(root *yaml.Node, key string, val *yaml.Node) {
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			root.Content[i+1] = val
			return
		}
	}
	root.Content = append(root.Content, scalarNode(key), val)
}

// mergeFrontmatterNode overlays the arc-owned keys onto the frontmatter mapping
// node, unioning tags. Untouched nodes are left exactly as parsed.
func mergeFrontmatterNode(root *yaml.Node, meta Frontmatter) {
	// Only overlay each arc-owned key when the incoming value is non-empty, so a
	// re-registration with a partially-populated meta (for example project="" when
	// registering from outside a known workspace) does not clobber good values.
	if meta.Title != "" {
		setMapEntry(root, "title", scalarNode(meta.Title))
	}
	if meta.Date != "" {
		setMapEntry(root, "date", scalarNode(meta.Date))
	}
	if meta.Project != "" {
		setMapEntry(root, "project", scalarNode(meta.Project))
	}
	if meta.Status != "" {
		setMapEntry(root, "status", scalarNode(meta.Status))
	}
	if meta.ArcReview.Kind != "" || meta.ArcReview.ID != "" {
		nested := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
			scalarNode("kind"), scalarNode(meta.ArcReview.Kind),
			scalarNode("id"), scalarNode(meta.ArcReview.ID),
		}}
		setMapEntry(root, "arc_review", nested)
	}
	setMapEntry(root, "tags", unionTagsNode(findMapValue(root, "tags"), meta.Tags))
}

// unionTagsNode merges the existing `tags` node (any YAML shape) with arc's tags
// into a fresh string sequence node, preserving existing order first and
// deduplicating. A nil existing node (key absent) contributes nothing.
func unionTagsNode(existing *yaml.Node, arcTags []string) *yaml.Node {
	seen := map[string]bool{}
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			seq.Content = append(seq.Content, scalarNode(s))
		}
	}
	if existing != nil {
		switch existing.Kind {
		case yaml.SequenceNode:
			for _, item := range existing.Content {
				// item.Value is the literal scalar text, so non-string list
				// items (e.g. `tags: [2024]`) are coerced rather than dropped.
				// Skip explicit nulls (`tags: [~]`).
				if item.Tag == "!!null" {
					continue
				}
				add(item.Value)
			}
		case yaml.ScalarNode:
			// Bare scalar (e.g. `tags: solo-tag`) — valid YAML, seen in
			// hand-edited vaults.
			if existing.Tag != "!!null" {
				add(existing.Value)
			}
		}
	}
	for _, s := range arcTags {
		add(s)
	}
	return seq
}

// SetStatus surgically replaces only the `status:` line in the leading frontmatter. Atomic.
// ErrNoFrontmatter (sentinel) if no frontmatter/status line — caller warns and continues.
func SetStatus(path, status string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.HasPrefix(raw, fmDelim) && !bytes.HasPrefix(raw, []byte("---\r\n")) {
		return ErrNoFrontmatter
	}
	lines := strings.SplitAfter(string(raw), "\n")
	end := -1
	for i := 1; i < len(lines); i++ {
		// Trim both \r and \n to handle CRLF (\r\n) and LF (\n) line endings.
		if strings.TrimRight(lines[i], "\r\n") == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return ErrNoFrontmatter
	}
	for i := 1; i < end; i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "status:") {
			// Preserve the original line ending (CRLF or LF).
			le := ""
			if strings.HasSuffix(lines[i], "\r\n") {
				le = "\r\n"
			} else if strings.HasSuffix(lines[i], "\n") {
				le = "\n"
			}
			lines[i] = "status: " + status + le
			return atomicWrite(path, []byte(strings.Join(lines, "")))
		}
	}
	return ErrNoFrontmatter
}

func atomicWrite(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".arcfm-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}
