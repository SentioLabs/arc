// Package config — docs-destination constants and resolution.
package config

import "fmt"

// Doc type values for plans.type / per-project docs.type.
const (
	DocsTypeMarkdown = "markdown"
	DocsTypeObsidian = "obsidian"
)

// Per-project config-table keys.
const (
	ProjectDocsPathKey = "docs.path"
	ProjectDocsTypeKey = "docs.type"
)

// Resolution sources reported alongside the resolved docs location.
const (
	DocsSourceProject = "project"
	DocsSourceConfig  = "config"
	DocsSourceDefault = "default"
)

// ValidDocsType reports whether t is an allowed docs type.
func ValidDocsType(t string) bool {
	return t == DocsTypeMarkdown || t == DocsTypeObsidian
}

// ResolveDocs returns the effective docs directory (absolute), doc type, and
// which layer supplied the directory. projectVals are the project's
// config-table rows (nil-safe); cfg is the loaded ~/.arc/config.toml (nil-safe);
// vars are the ExpandPlansDir template variables ({project}, {prefix}); cwd
// anchors relative paths. Order: docs.* project rows → [plans] config → defaults.
func ResolveDocs(projectVals map[string]string, cfg *Config, vars map[string]string, cwd string) (path, docType, source string, err error) {
	def := Default()

	docType = def.Plans.Type
	if cfg != nil && cfg.Plans.Type != "" {
		docType = cfg.Plans.Type
	}
	if t := projectVals[ProjectDocsTypeKey]; t != "" {
		docType = t
	}
	if !ValidDocsType(docType) {
		return "", "", "", fmt.Errorf("invalid docs type %q (want %q or %q)", docType, DocsTypeMarkdown, DocsTypeObsidian)
	}

	tmpl, source := def.Plans.Dir, DocsSourceDefault
	if cfg != nil && cfg.Plans.Dir != "" && cfg.Plans.Dir != def.Plans.Dir {
		tmpl, source = cfg.Plans.Dir, DocsSourceConfig
	}
	if p := projectVals[ProjectDocsPathKey]; p != "" {
		tmpl, source = p, DocsSourceProject
	}

	path, err = ExpandPlansDir(tmpl, vars, cwd)
	if err != nil {
		return "", "", "", err
	}
	return path, docType, source, nil
}
