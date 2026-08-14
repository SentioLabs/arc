package config_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sentiolabs/arc/internal/config"
)

// --- Contract assertions ---
// These verify the design spec. Do NOT modify without updating the approved plan.

var _ func(map[string]string, *config.Config, map[string]string, string) (
	string, string, string, error,
) = config.ResolveDocs

func TestDocsConstantsContract(t *testing.T) {
	if config.DocsTypeMarkdown != "markdown" || config.DocsTypeObsidian != "obsidian" {
		t.Fatal("docs type constants changed")
	}
	if config.ProjectDocsPathKey != "docs.path" || config.ProjectDocsTypeKey != "docs.type" {
		t.Fatal("project config key constants changed")
	}
	if config.DocsSourceProject != "project" || config.DocsSourceConfig != "config" ||
		config.DocsSourceDefault != "default" {
		t.Fatal("source constants changed")
	}
}

// --- Behavior tests ---

func TestResolveDocsDefault(t *testing.T) {
	path, dtype, source, err := config.ResolveDocs(nil, config.Default(), map[string]string{}, "/tmp/repo")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) || !strings.HasSuffix(path, filepath.Join("docs", "plans")) {
		t.Fatalf("want absolute .../docs/plans, got %q", path)
	}
	if dtype != config.DocsTypeMarkdown || source != config.DocsSourceDefault {
		t.Fatalf("got type=%q source=%q", dtype, source)
	}
}

func TestResolveDocsConfigLayer(t *testing.T) {
	cfg := config.Default()
	cfg.Plans.Dir = "~/vault/{project}"
	cfg.Plans.Type = config.DocsTypeObsidian
	vars := map[string]string{"project": "myproj", "prefix": "mp"}
	path, dtype, source, err := config.ResolveDocs(nil, cfg, vars, "/tmp/repo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, filepath.Join("vault", "myproj")) {
		t.Fatalf("template not expanded: %q", path)
	}
	if dtype != config.DocsTypeObsidian || source != config.DocsSourceConfig {
		t.Fatalf("got type=%q source=%q", dtype, source)
	}
}

func TestResolveDocsProjectOverrideWins(t *testing.T) {
	cfg := config.Default()
	cfg.Plans.Dir = "~/vault/{project}"
	rows := map[string]string{
		config.ProjectDocsPathKey: "/abs/other",
		config.ProjectDocsTypeKey: config.DocsTypeObsidian,
	}
	vars := map[string]string{"project": "p", "prefix": "x"}
	path, dtype, source, err := config.ResolveDocs(rows, cfg, vars, "/tmp/repo")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/abs/other" || dtype != config.DocsTypeObsidian || source != config.DocsSourceProject {
		t.Fatalf("got path=%q type=%q source=%q", path, dtype, source)
	}
}

func TestResolveDocsInvalidType(t *testing.T) {
	rows := map[string]string{config.ProjectDocsTypeKey: "notion"}
	if _, _, _, err := config.ResolveDocs(rows, config.Default(), map[string]string{}, "/tmp"); err == nil {
		t.Fatal("want error for invalid docs type")
	}
}
