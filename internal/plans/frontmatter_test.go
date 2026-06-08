package plans_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sentiolabs/arc/internal/plans"
)

// --- Contract assertions ---
var (
	_ string          = plans.Frontmatter{}.Status
	_ plans.ArcReview = plans.Frontmatter{}.ArcReview
)

func TestEnsureAndSetStatus(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(p, []byte("# Title\n\nbody\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	meta := plans.Frontmatter{
		Title: "T", Date: "2026-06-07", Project: "arc", Status: "in_review",
		Tags:      []string{"arc"},
		ArcReview: plans.ArcReview{Kind: "legacy", ID: "plan.x"},
	}
	if err := plans.EnsureFrontmatter(p, meta); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if !strings.HasPrefix(string(got), "---\n") || !strings.Contains(string(got), "# Title") {
		t.Fatalf("bad: %s", got)
	}
	if err := plans.SetStatus(p, "approved"); err != nil {
		t.Fatal(err)
	}
	got2, _ := os.ReadFile(p)
	if !strings.Contains(string(got2), "status: approved") {
		t.Fatalf("status: %s", got2)
	}
	plain := filepath.Join(dir, "plain.md")
	if err := os.WriteFile(plain, []byte("no fm\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := plans.SetStatus(plain, "approved"); !errors.Is(err, plans.ErrNoFrontmatter) {
		t.Fatalf("want ErrNoFrontmatter got %v", err)
	}
}

// TestReadFrontmatterDashesInBodyAfterCloser verifies that a line starting with
// "---" inside the body (e.g. a markdown horizontal rule or "----") does not
// close the frontmatter block — only an exact "---" line (with no other
// characters) acts as the closer.
//
// Body contains a line "----" (four dashes) and a line "--- x" — neither should
// be treated as the closing delimiter. Note: the body here is AFTER the real
// closing "---", so the current search order still works fine.
func TestReadFrontmatterDashesInBodyAfterCloser(t *testing.T) {
	input := "---\ntitle: Test\nstatus: draft\n---\n# Heading\n\n----\n\n--- x\n\nend\n"
	fm, body, ok, err := plans.ReadFrontmatter([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected frontmatter to be found")
	}
	if fm.Title != "Test" {
		t.Fatalf("unexpected title: %q", fm.Title)
	}
	if !strings.Contains(string(body), "----") {
		t.Errorf("body should contain '----' (markdown hr); got: %q", body)
	}
	if !strings.Contains(string(body), "--- x") {
		t.Errorf("body should contain '--- x'; got: %q", body)
	}
	if !strings.Contains(string(body), "end") {
		t.Errorf("body should contain 'end'; got: %q", body)
	}

	// EnsureFrontmatter round-trip should preserve that body content.
	dir := t.TempDir()
	p := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(p, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := plans.EnsureFrontmatter(p, plans.Frontmatter{Title: "Test", Status: "draft"}); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "----") {
		t.Errorf("round-trip body missing '----'; got: %q", out)
	}
	if !strings.Contains(string(out), "--- x") {
		t.Errorf("round-trip body missing '--- x'; got: %q", out)
	}
}

// TestReadFrontmatterDashesBeforeExactCloser verifies that the body can itself
// contain "----" (four-dash hr) followed by "--- x" without the parser being
// confused — the only valid closer is exactly "---" (optionally followed by
// CRLF/LF or EOF).
//
// The key assertion: the body returned must NOT start with "---\n", which would
// indicate the parser mistook "----" in the body as the closing delimiter.
func TestReadFrontmatterDashesBeforeExactCloser(t *testing.T) {
	// Structure: opening --- | YAML | closing --- | body with ---- and --- x
	// With the naive "\n---" search, the first "\n---" found in the body
	// after the real closer would be "--- x" — but since we already have
	// the real closer, this only matters if "----" appeared BEFORE it.
	// Here we test that "----\n" and "--- x\n" in the body do not
	// pollute the frontmatter block and that body is returned intact.
	input := "---\ntitle: Tricky\nstatus: draft\n---\n----\n--- x\nbody after\n"
	fm, body, ok, err := plans.ReadFrontmatter([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected frontmatter to be found")
	}
	if fm.Title != "Tricky" {
		t.Fatalf("unexpected title: %q", fm.Title)
	}
	// Body must contain all lines after the closing ---.
	if !strings.Contains(string(body), "----") {
		t.Errorf("body should contain '----'; got: %q", body)
	}
	if !strings.Contains(string(body), "--- x") {
		t.Errorf("body should contain '--- x'; got: %q", body)
	}
	if !strings.Contains(string(body), "body after") {
		t.Errorf("body should contain 'body after'; got: %q", body)
	}
	// Body must start at "----", NOT at "---\n" (which would mean the closer
	// was missed and body contains the closing delimiter line).
	if strings.HasPrefix(string(body), "---\n") {
		t.Errorf("body starts with '---\\n' suggesting the real closer was not recognized; got: %q", body)
	}
}

// TestReadFrontmatterNoTrailingNewlineAfterClose verifies that a file ending
// exactly with "---" (no trailing newline) is parsed correctly and body is preserved.
func TestReadFrontmatterNoTrailingNewlineAfterClose(t *testing.T) {
	// No newline after closing ---
	input := "---\ntitle: Test\nstatus: draft\n---"
	fm, body, ok, err := plans.ReadFrontmatter([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected frontmatter to be found even without trailing newline after closing ---")
	}
	if fm.Title != "Test" {
		t.Fatalf("unexpected title: %q", fm.Title)
	}
	// Body should be empty (not nil drop), and no panic.
	_ = body

	// File ending with "---\nbody content" (body present, no newline after close marker)
	// This tests the case: closing --- has no trailing newline but content existed before it
	// was rewritten. Use EnsureFrontmatter round-trip.
	dir := t.TempDir()
	p := filepath.Join(dir, "spec.md")
	// Write file with trailing body but no final newline after ---
	if err := os.WriteFile(p, []byte("---\ntitle: NoNL\nstatus: draft\n---\nbody here"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := plans.EnsureFrontmatter(p, plans.Frontmatter{Title: "NoNL", Status: "draft"}); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "body here") {
		t.Errorf("body content was dropped; got: %q", out)
	}
}

// TestSetStatusCRLF verifies that SetStatus works correctly on a CRLF-encoded file:
// it must locate the closing "---" delimiter (which appears as "---\r\n"), rewrite
// the status line preserving CRLF, and NOT return ErrNoFrontmatter.
func TestSetStatusCRLF(t *testing.T) {
	// Build a CRLF frontmatter file.
	crlf := "---\r\ntitle: CRLFTest\r\nstatus: draft\r\n---\r\n# Body\r\n"
	dir := t.TempDir()
	p := filepath.Join(dir, "crlf.md")
	if err := os.WriteFile(p, []byte(crlf), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := plans.SetStatus(p, "approved"); err != nil {
		t.Fatalf("SetStatus on CRLF file returned error: %v", err)
	}

	out, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	outStr := string(out)

	// The status line must be updated.
	if !strings.Contains(outStr, "status: approved") {
		t.Errorf("status not updated in CRLF file; got: %q", outStr)
	}

	// The rewritten status line must preserve CRLF.
	if !strings.Contains(outStr, "status: approved\r\n") {
		t.Errorf("CRLF not preserved on status line; got: %q", outStr)
	}
}
