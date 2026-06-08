// Package config provides configuration loading, validation, and template
// expansion for the arc CLI. Template variables use {name} syntax and are
// expanded by ExpandPlansDir before any path is used at runtime.
package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// templateVarRe matches {identifier} placeholders in template strings.
var templateVarRe = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

// slugStripRe matches any run of characters that are not lowercase alphanumeric.
var slugStripRe = regexp.MustCompile(`[^a-z0-9]+`)

// SanitizeSlug lowercases s, replaces runs of non [a-z0-9] with '-', trims '-'. "" if nothing survives.
func SanitizeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugStripRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// ExpandPlansDir substitutes {vars}, expands leading ~, returns an absolute dir
// (relative resolved against cwd). Unknown {placeholder} or empty substitution => error.
func ExpandPlansDir(tmpl string, vars map[string]string, cwd string) (string, error) {
	var badVar, emptyVar string
	out := templateVarRe.ReplaceAllStringFunc(tmpl, func(m string) string {
		name := m[1 : len(m)-1]
		v, ok := vars[name]
		if !ok {
			badVar = name
			return m
		}
		if v == "" {
			emptyVar = name
		}
		return v
	})
	if badVar != "" {
		return "", fmt.Errorf("unknown template variable {%s} in plans.dir", badVar)
	}
	if emptyVar != "" {
		return "", fmt.Errorf("template variable {%s} expanded to empty", emptyVar)
	}
	out = expandHome(out)
	if !filepath.IsAbs(out) {
		out = filepath.Join(cwd, out)
	}
	return out, nil
}
