package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeriveTitle_H1Heading(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "my-spec.md")
	content := "# My Spec Title\n\nSome body text here.\n"
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	got := deriveTitle(f)
	want := "My Spec Title"
	if got != want {
		t.Errorf("deriveTitle H1 case: got %q, want %q", got, want)
	}
}

func TestDeriveTitle_FilenameFallback(t *testing.T) {
	dir := t.TempDir()
	// File with a YYYY-MM-DD- date prefix, no H1 heading
	f := filepath.Join(dir, "2024-01-15-my-design-spec.md")
	content := "Some content without a heading.\n"
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	got := deriveTitle(f)
	want := "my-design-spec"
	if got != want {
		t.Errorf("deriveTitle filename fallback: got %q, want %q", got, want)
	}
}

func TestDeriveTitle_NonExistentPath(t *testing.T) {
	dir := t.TempDir()
	// Path that does not exist — should fall back to filename base (sans date prefix / .md)
	f := filepath.Join(dir, "2024-03-01-missing-plan.md")

	got := deriveTitle(f)
	want := "missing-plan"
	if got != want {
		t.Errorf("deriveTitle non-existent path: got %q, want %q", got, want)
	}
}

func TestDeriveTitle_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	// Empty file — no heading, should fall back to filename base
	f := filepath.Join(dir, "2024-05-10-empty-spec.md")
	if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	got := deriveTitle(f)
	want := "empty-spec"
	if got != want {
		t.Errorf("deriveTitle empty file: got %q, want %q", got, want)
	}
}

func TestDeriveTitle_H2OnlyHeading(t *testing.T) {
	dir := t.TempDir()
	// File whose only heading is ## (H2) — should NOT match, fall back to filename
	f := filepath.Join(dir, "2024-06-01-h2-only.md")
	content := "## Not An H1\n\nBody text.\n"
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	got := deriveTitle(f)
	want := "h2-only"
	if got != want {
		t.Errorf("deriveTitle H2-only heading: got %q, want %q", got, want)
	}
}
