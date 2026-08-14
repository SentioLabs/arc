package config

import (
	"path/filepath"
	"strings"
	"testing"
)

// --- Contract assertions ---
// These verify the design spec. Do NOT modify without updating the approved plan.

var _ func(map[string]string, *Config, map[string]string, string) (string, string, string, error) = ResolveDocs

func TestDocsConstantsContract(t *testing.T) {
	if DocsTypeMarkdown != "markdown" || DocsTypeObsidian != "obsidian" {
		t.Fatal("docs type constants changed")
	}
	if ProjectDocsPathKey != "docs.path" || ProjectDocsTypeKey != "docs.type" {
		t.Fatal("project config key constants changed")
	}
	if DocsSourceProject != "project" || DocsSourceConfig != "config" || DocsSourceDefault != "default" {
		t.Fatal("source constants changed")
	}
}

// --- Behavior tests ---

func TestResolveDocsDefault(t *testing.T) {
	path, dtype, source, err := ResolveDocs(nil, Default(), map[string]string{}, "/tmp/repo")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) || !strings.HasSuffix(path, filepath.Join("docs", "plans")) {
		t.Fatalf("want absolute .../docs/plans, got %q", path)
	}
	if dtype != DocsTypeMarkdown || source != DocsSourceDefault {
		t.Fatalf("got type=%q source=%q", dtype, source)
	}
}

func TestResolveDocsConfigLayer(t *testing.T) {
	cfg := Default()
	cfg.Plans.Dir = "~/vault/{project}"
	cfg.Plans.Type = DocsTypeObsidian
	path, dtype, source, err := ResolveDocs(nil, cfg, map[string]string{"project": "myproj", "prefix": "mp"}, "/tmp/repo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, filepath.Join("vault", "myproj")) {
		t.Fatalf("template not expanded: %q", path)
	}
	if dtype != DocsTypeObsidian || source != DocsSourceConfig {
		t.Fatalf("got type=%q source=%q", dtype, source)
	}
}

func TestResolveDocsProjectOverrideWins(t *testing.T) {
	cfg := Default()
	cfg.Plans.Dir = "~/vault/{project}"
	rows := map[string]string{ProjectDocsPathKey: "/abs/other", ProjectDocsTypeKey: DocsTypeObsidian}
	path, dtype, source, err := ResolveDocs(rows, cfg, map[string]string{"project": "p", "prefix": "x"}, "/tmp/repo")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/abs/other" || dtype != DocsTypeObsidian || source != DocsSourceProject {
		t.Fatalf("got path=%q type=%q source=%q", path, dtype, source)
	}
}

func TestResolveDocsInvalidType(t *testing.T) {
	rows := map[string]string{ProjectDocsTypeKey: "notion"}
	if _, _, _, err := ResolveDocs(rows, Default(), map[string]string{}, "/tmp"); err == nil {
		t.Fatal("want error for invalid docs type")
	}
}
