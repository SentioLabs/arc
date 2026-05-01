package config

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// ValidChannels lists the allowed values for updates.channel.
var ValidChannels = []string{"stable", "rc", "nightly"}

// ValidationErrors maps dotted-key to error message.
type ValidationErrors map[string]string

func (v ValidationErrors) Error() string {
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

// Validate returns a ValidationErrors if cfg has invalid values, or nil if OK.
func Validate(cfg *Config) error {
	errs := ValidationErrors{}
	if cfg.CLI.Server != "" {
		if u, err := url.Parse(cfg.CLI.Server); err != nil || u.Scheme == "" || u.Host == "" {
			errs["cli.server"] = "must be a valid URL with scheme and host"
		}
	}
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		errs["server.port"] = "must be between 1 and 65535"
	}
	if cfg.Share.Server != "" {
		if u, err := url.Parse(cfg.Share.Server); err != nil || u.Scheme == "" || u.Host == "" {
			errs["share.server"] = "must be a valid URL with scheme and host"
		}
	}
	channelOK := false
	for _, c := range ValidChannels {
		if cfg.Updates.Channel == c {
			channelOK = true
			break
		}
	}
	if !channelOK {
		errs["updates.channel"] = fmt.Sprintf("must be one of: %s", strings.Join(ValidChannels, ", "))
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}
