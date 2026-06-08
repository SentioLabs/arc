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

// ReadFrontmatter parses a leading --- block. ok=false if absent (legacy doc).
func ReadFrontmatter(b []byte) (fm Frontmatter, body []byte, ok bool, err error) {
	if !bytes.HasPrefix(b, fmDelim) {
		return Frontmatter{}, b, false, nil
	}
	rest := b[len(fmDelim):]
	end := bytes.Index(rest, []byte("\n---"))
	if end < 0 {
		return Frontmatter{}, b, false, nil
	}
	if err := yaml.Unmarshal(rest[:end], &fm); err != nil {
		return Frontmatter{}, b, false, err
	}
	after := rest[end+1:]
	if i := bytes.IndexByte(after, '\n'); i >= 0 {
		body = after[i+1:]
	}
	return fm, body, true, nil
}

// EnsureFrontmatter idempotently writes the arc-owned frontmatter block, preserving body. Atomic.
func EnsureFrontmatter(path string, meta Frontmatter) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, body, ok, err := ReadFrontmatter(raw)
	if err != nil {
		return err
	}
	if !ok {
		body = raw
	}
	y, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.Write(fmDelim)
	buf.Write(y)
	buf.WriteString("---\n")
	buf.Write(body)
	return atomicWrite(path, buf.Bytes())
}

// SetStatus surgically replaces only the `status:` line in the leading frontmatter. Atomic.
// ErrNoFrontmatter (sentinel) if no frontmatter/status line — caller warns and continues.
func SetStatus(path, status string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.HasPrefix(raw, fmDelim) {
		return ErrNoFrontmatter
	}
	lines := strings.SplitAfter(string(raw), "\n")
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\n") == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return ErrNoFrontmatter
	}
	for i := 1; i < end; i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "status:") {
			nl := ""
			if strings.HasSuffix(lines[i], "\n") {
				nl = "\n"
			}
			lines[i] = "status: " + status + nl
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
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}
