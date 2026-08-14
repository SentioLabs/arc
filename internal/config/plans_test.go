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
) = config.ResolvePlans

func TestPlansConstantsContract(t *testing.T) {
	if config.PlansTypeMarkdown != "markdown" || config.PlansTypeObsidian != "obsidian" {
		t.Fatal("plans type constants changed")
	}
	if config.ProjectPlansDirKey != "plans.dir" || config.ProjectPlansTypeKey != "plans.type" {
		t.Fatal("project config key constants changed")
	}
	if config.PlansSourceProject != "project" || config.PlansSourceConfig != "config" ||
		config.PlansSourceDefault != "default" {
		t.Fatal("source constants changed")
	}
}

// --- Behavior tests ---

func TestResolvePlansDefault(t *testing.T) {
	dir, ptype, source, err := config.ResolvePlans(nil, config.Default(), map[string]string{}, "/tmp/repo")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(dir) || !strings.HasSuffix(dir, filepath.Join("docs", "plans")) {
		t.Fatalf("want absolute .../docs/plans, got %q", dir)
	}
	if ptype != config.PlansTypeMarkdown || source != config.PlansSourceDefault {
		t.Fatalf("got type=%q source=%q", ptype, source)
	}
}

func TestResolvePlansConfigLayer(t *testing.T) {
	cfg := config.Default()
	cfg.Plans.Dir = "~/vault/{project}"
	cfg.Plans.Type = config.PlansTypeObsidian
	vars := map[string]string{"project": "myproj", "prefix": "mp"}
	dir, ptype, source, err := config.ResolvePlans(nil, cfg, vars, "/tmp/repo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(dir, filepath.Join("vault", "myproj")) {
		t.Fatalf("template not expanded: %q", dir)
	}
	if ptype != config.PlansTypeObsidian || source != config.PlansSourceConfig {
		t.Fatalf("got type=%q source=%q", ptype, source)
	}
}

func TestResolvePlansProjectOverrideWins(t *testing.T) {
	cfg := config.Default()
	cfg.Plans.Dir = "~/vault/{project}"
	rows := map[string]string{
		config.ProjectPlansDirKey:  "/abs/other",
		config.ProjectPlansTypeKey: config.PlansTypeObsidian,
	}
	vars := map[string]string{"project": "p", "prefix": "x"}
	dir, ptype, source, err := config.ResolvePlans(rows, cfg, vars, "/tmp/repo")
	if err != nil {
		t.Fatal(err)
	}
	if dir != "/abs/other" || ptype != config.PlansTypeObsidian || source != config.PlansSourceProject {
		t.Fatalf("got dir=%q type=%q source=%q", dir, ptype, source)
	}
}

func TestResolvePlansInvalidType(t *testing.T) {
	rows := map[string]string{config.ProjectPlansTypeKey: "notion"}
	if _, _, _, err := config.ResolvePlans(rows, config.Default(), map[string]string{}, "/tmp"); err == nil {
		t.Fatal("want error for invalid plans type")
	}
}
