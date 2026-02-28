// Package main provides the config management commands for the arc CLI,
// allowing users to view and modify CLI configuration values such as the
// server URL.
//
// The config subcommands (list, get, set, path) read and write
// the JSON configuration stored at ~/.arc/cli-config.json. The only
// recognised key today is "server_url" which points at the arc server
// that the CLI talks to.
package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// configSetArgCount is the number of arguments required for the config set command.
const configSetArgCount = 2

// validConfigKeys lists the allowed configuration keys.
var validConfigKeys = []string{"server_url"}

// configCmd is the parent command for CLI configuration management.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long: `View and modify arc CLI configuration.

Configuration is stored in ~/.arc/cli-config.json`,
}

// configListCmd prints every config key and its current value.
var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration values",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(cfg)
			return nil
		}

		_, _ = fmt.Printf("server_url = %s\n", cfg.ServerURL)
		_, _ = fmt.Printf("\nConfig file: %s\n", defaultConfigPath())
		return nil
	},
}

// configGetCmd retrieves a single configuration value by key.
var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Long: `Get a configuration value.

Valid keys: server_url`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := strings.ToLower(args[0])

		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		var value string
		switch key {
		case "server_url", "server-url", "serverurl":
			value = cfg.ServerURL
		default:
			return fmt.Errorf("unknown config key: %s\nValid keys: %s", key, strings.Join(validConfigKeys, ", "))
		}

		if outputJSON {
			outputResult(map[string]string{key: value})
			return nil
		}

		fmt.Println(value)
		return nil
	},
}

// configSetCmd persists a new value for a configuration key.
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value.

Valid keys: server_url

Examples:
  arc config set server_url http://localhost:7432
  arc config set server_url http://remote-server:7432`,
	Args: cobra.ExactArgs(configSetArgCount),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := strings.ToLower(args[0])
		value := args[1]

		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		switch key {
		case "server_url", "server-url", "serverurl":
			cfg.ServerURL = value
		default:
			return fmt.Errorf("unknown config key: %s\nValid keys: %s", key, strings.Join(validConfigKeys, ", "))
		}

		if err := saveConfig(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		_, _ = fmt.Printf("Set %s = %s\n", key, value)
		return nil
	},
}

// configPathCmd displays the filesystem path to the active config file.
var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show the config file path",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := configPath
		if path == "" {
			path = defaultConfigPath()
		}
		fmt.Println(path)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configPathCmd)

	rootCmd.AddCommand(configCmd)
}
