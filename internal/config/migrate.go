package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type legacyJSON struct {
	ServerURL string `json:"server_url"`
	Channel   string `json:"channel"`
}

// migrateLegacyJSON reads the flat ~/.arc/cli-config.json shape and maps it
// onto the new Config. Old keys map as:
//
//	server_url → cli.server
//	channel    → updates.channel
//
// Unknown keys are ignored.
func migrateLegacyJSON(jsonPath string) (*Config, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("read legacy json: %w", err)
	}
	var legacy legacyJSON
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("parse legacy json: %w", err)
	}
	cfg := Default()
	if legacy.ServerURL != "" {
		cfg.CLI.Server = legacy.ServerURL
	}
	if legacy.Channel != "" {
		cfg.Updates.Channel = legacy.Channel
	}
	return cfg, nil
}
