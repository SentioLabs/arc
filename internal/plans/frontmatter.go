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

var fmDelim = []byte("---\n")

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
// --- block is present (legacy doc).
func readRawFrontmatter(b []byte) (fm []byte, body []byte, ok bool, err error) {
	if !bytes.HasPrefix(b, fmDelim) {
		return nil, b, false, nil
	}
	rest := b[len(fmDelim):]
	end := findClosingDelim(rest)
	if end < 0 {
		return nil, b, false, nil
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
	return fm, body, true, nil
}

// ReadFrontmatter parses a leading --- block. ok=false if absent (legacy doc).
func ReadFrontmatter(b []byte) (fm Frontmatter, body []byte, ok bool, err error) {
	raw, body, ok, err := readRawFrontmatter(b)
	if err != nil || !ok {
		return Frontmatter{}, body, ok, err
	}
	if err := yaml.Unmarshal(raw, &fm); err != nil {
		return Frontmatter{}, b, false, err
	}
	return fm, body, true, nil
}

// EnsureFrontmatter idempotently merges the arc-owned frontmatter keys into the
// file's existing frontmatter (creating it if absent), preserving all other
// keys and unioning tags. Atomic.
func EnsureFrontmatter(path string, meta Frontmatter) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	existing := map[string]any{}
	body := raw
	if fm, rest, ok, rerr := readRawFrontmatter(raw); rerr == nil && ok {
		if uerr := yaml.Unmarshal(fm, &existing); uerr != nil {
			return uerr
		}
		body = rest
	}

	merged := mergeFrontmatter(existing, meta)

	y, err := yaml.Marshal(merged)
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

// mergeFrontmatter overlays the arc-owned keys onto existing, unioning tags.
func mergeFrontmatter(existing map[string]any, meta Frontmatter) map[string]any {
	if existing == nil {
		existing = map[string]any{}
	}
	existing["title"] = meta.Title
	existing["date"] = meta.Date
	existing["project"] = meta.Project
	existing["status"] = meta.Status
	existing["arc_review"] = map[string]any{"kind": meta.ArcReview.Kind, "id": meta.ArcReview.ID}
	existing["tags"] = unionTags(existing["tags"], meta.Tags)
	return existing
}

// unionTags merges existing frontmatter tags (any YAML shape) with arc's tags,
// preserving existing order first, deduplicated.
func unionTags(existing any, arcTags []string) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	switch v := existing.(type) {
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				add(s)
			}
		}
	case string:
		// Bare scalar (e.g. `tags: solo-tag`) — valid YAML, seen in hand-edited vaults.
		add(v)
	}
	for _, s := range arcTags {
		add(s)
	}
	return out
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
