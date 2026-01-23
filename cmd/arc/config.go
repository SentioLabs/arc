package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Valid config keys
var validConfigKeys = []string{"server_url"}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long: `View and modify arc CLI configuration.

Configuration is stored in ~/.arc/cli-config.json`,
}

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

		fmt.Printf("server_url = %s\n", cfg.ServerURL)
		fmt.Printf("\nConfig file: %s\n", defaultConfigPath())
		return nil
	},
}

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

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value.

Valid keys: server_url

Examples:
  arc config set server_url http://localhost:7432
  arc config set server_url http://remote-server:7432`,
	Args: cobra.ExactArgs(2),
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

		fmt.Printf("Set %s = %s\n", key, value)
		return nil
	},
}

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
