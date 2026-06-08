package config

import (
	"strings"
	"testing"
)

// --- Contract assertions ---
var _ = Config{}.Plans.Dir

func TestSanitizeSlug(t *testing.T) {
	for in, want := range map[string]string{"My App": "my-app", "  Foo__Bar  ": "foo-bar", "!!!": ""} {
		if got := SanitizeSlug(in); got != want {
			t.Fatalf("SanitizeSlug(%q)=%q want %q", in, got, want)
		}
	}
}

func TestExpandPlansDir(t *testing.T) {
	got, err := ExpandPlansDir("~/V/{project}", map[string]string{"project": "arc"}, "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, "/V/arc") {
		t.Fatalf("got %q", got)
	}
	if _, err := ExpandPlansDir("~/V/{nope}", map[string]string{}, "/tmp"); err == nil {
		t.Fatal("want unknown-var error")
	}
	if _, err := ExpandPlansDir("~/V/{project}", map[string]string{"project": ""}, "/tmp"); err == nil {
		t.Fatal("want empty-var error")
	}
	if rel, err := ExpandPlansDir("docs/plans", map[string]string{}, "/tmp"); err != nil || rel != "/tmp/docs/plans" {
		t.Fatalf("rel=%q err=%v", rel, err)
	}
}
