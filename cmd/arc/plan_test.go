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
