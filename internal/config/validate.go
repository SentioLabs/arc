// Package config provides configuration loading, validation, and template
// expansion. Validate checks all fields and returns a ValidationError that
// describes every invalid field so callers can surface them together.
package config

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// Allowed template variable names for plans.dir.
const (
	tmplVarProject = "project"
	tmplVarPrefix  = "prefix"
)

// ValidChannels lists the allowed values for updates.channel.
var ValidChannels = []string{"stable", "rc", "nightly"}

// ValidationError maps dotted-key to error message.
type ValidationError map[string]string

// Error returns a sorted, semicolon-separated list of all validation failures.
func (v ValidationError) Error() string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %s", k, v[k]))
	}
	return "config validation failed: " + strings.Join(parts, "; ")
}

// Validate checks each field in cfg and returns a ValidationError describing all
// invalid fields, or nil if the config is fully valid.
func Validate(cfg *Config) error {
	errs := ValidationError{}
	if u, err := url.Parse(cfg.CLI.Server); err != nil || u.Scheme == "" || u.Host == "" {
		errs["cli.server"] = "must be a valid URL with scheme and host"
	}
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		errs["server.port"] = "must be between 1 and 65535"
	}
	if u, err := url.Parse(cfg.Share.Server); err != nil || u.Scheme == "" || u.Host == "" {
		errs["share.server"] = "must be a valid URL with scheme and host"
	}
	// Check that updates.channel is one of the allowed values.
	channelOK := false
	for _, c := range ValidChannels {
		if cfg.Updates.Channel == c {
			channelOK = true
			break
		}
	}
	if !channelOK {
		errs["updates.channel"] = "must be one of: " + strings.Join(ValidChannels, ", ")
	}
	switch {
	case cfg.Plans.Dir == "":
		errs["plans.dir"] = "must not be empty"
	case strings.Contains(cfg.Plans.Dir, ".."):
		errs["plans.dir"] = "must not contain '..'"
	default:
		for _, m := range templateVarRe.FindAllStringSubmatch(cfg.Plans.Dir, -1) {
			if m[1] != tmplVarProject && m[1] != tmplVarPrefix {
				errs["plans.dir"] = "unknown template variable {" + m[1] +
					"} (allowed: " + tmplVarProject + ", " + tmplVarPrefix + ")"
				break
			}
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}
