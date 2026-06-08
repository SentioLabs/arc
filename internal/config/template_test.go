package config_test

import (
	"strings"
	"testing"

	"github.com/sentiolabs/arc/internal/config"
)

// --- Contract assertions ---
var _ = config.Config{}.Plans.Dir

func TestSanitizeSlug(t *testing.T) {
	for in, want := range map[string]string{"My App": "my-app", "  Foo__Bar  ": "foo-bar", "!!!": ""} {
		if got := config.SanitizeSlug(in); got != want {
			t.Fatalf("SanitizeSlug(%q)=%q want %q", in, got, want)
		}
	}
}

func TestExpandPlansDir(t *testing.T) {
	got, err := config.ExpandPlansDir("~/V/{project}", map[string]string{"project": "arc"}, "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, "/V/arc") {
		t.Fatalf("got %q", got)
	}
	if _, err := config.ExpandPlansDir("~/V/{nope}", map[string]string{}, "/tmp"); err == nil {
		t.Fatal("want unknown-var error")
	}
	if _, err := config.ExpandPlansDir("~/V/{project}", map[string]string{"project": ""}, "/tmp"); err == nil {
		t.Fatal("want empty-var error")
	}
	rel, relErr := config.ExpandPlansDir("docs/plans", map[string]string{}, "/tmp")
	if relErr != nil || rel != "/tmp/docs/plans" {
		t.Fatalf("rel=%q err=%v", rel, relErr)
	}
}
