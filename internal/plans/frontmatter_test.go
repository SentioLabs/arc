package plans

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Contract assertions ---
var _ string = Frontmatter{}.Status
var _ ArcReview = Frontmatter{}.ArcReview

func TestEnsureAndSetStatus(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "spec.md")
	os.WriteFile(p, []byte("# Title\n\nbody\n"), 0o644)
	if err := EnsureFrontmatter(p, Frontmatter{Title: "T", Date: "2026-06-07", Project: "arc", Status: "in_review", Tags: []string{"arc"}, ArcReview: ArcReview{Kind: "legacy", ID: "plan.x"}}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if !strings.HasPrefix(string(got), "---\n") || !strings.Contains(string(got), "# Title") {
		t.Fatalf("bad: %s", got)
	}
	if err := SetStatus(p, "approved"); err != nil {
		t.Fatal(err)
	}
	got2, _ := os.ReadFile(p)
	if !strings.Contains(string(got2), "status: approved") {
		t.Fatalf("status: %s", got2)
	}
	plain := filepath.Join(dir, "plain.md")
	os.WriteFile(plain, []byte("no fm\n"), 0o644)
	if err := SetStatus(plain, "approved"); err != ErrNoFrontmatter {
		t.Fatalf("want ErrNoFrontmatter got %v", err)
	}
}
