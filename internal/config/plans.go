// plans-destination constants and resolution.
package config

import "fmt"

// Plan type values for plans.type / per-project plans.type.
const (
	PlansTypeMarkdown = "markdown"
	PlansTypeObsidian = "obsidian"
)

// Per-project config-table keys.
const (
	ProjectPlansDirKey  = "plans.dir"
	ProjectPlansTypeKey = "plans.type"
)

// Resolution sources reported alongside the resolved plans location.
const (
	PlansSourceProject = "project"
	PlansSourceConfig  = "config"
	PlansSourceDefault = "default"
)

// ValidPlansType reports whether t is an allowed plans type.
func ValidPlansType(t string) bool {
	return t == PlansTypeMarkdown || t == PlansTypeObsidian
}

// ResolvePlans returns the effective plans directory (absolute), plan type, and
// which layer supplied the directory. projectVals are the project's
// config-table rows (nil-safe); cfg is the loaded ~/.arc/config.toml
// (nil-safe); vars are the ExpandPlansDir template variables ({project},
// {prefix}); cwd anchors relative paths. Order: plans.* project rows →
// [plans] config → defaults.
func ResolvePlans(projectVals map[string]string, cfg *Config, vars map[string]string, cwd string) (
	dir, planType, source string, err error,
) {
	def := Default()

	planType = def.Plans.Type
	if cfg != nil && cfg.Plans.Type != "" {
		planType = cfg.Plans.Type
	}
	if t := projectVals[ProjectPlansTypeKey]; t != "" {
		planType = t
	}
	if !ValidPlansType(planType) {
		return "", "", "", fmt.Errorf("invalid plans type %q (want %q or %q)",
			planType, PlansTypeMarkdown, PlansTypeObsidian)
	}

	tmpl, source := def.Plans.Dir, PlansSourceDefault
	if cfg != nil && cfg.Plans.Dir != "" && cfg.Plans.Dir != def.Plans.Dir {
		tmpl, source = cfg.Plans.Dir, PlansSourceConfig
	}
	if p := projectVals[ProjectPlansDirKey]; p != "" {
		tmpl, source = p, PlansSourceProject
	}

	dir, err = ExpandPlansDir(tmpl, vars, cwd)
	if err != nil {
		return "", "", "", err
	}
	return dir, planType, source, nil
}
