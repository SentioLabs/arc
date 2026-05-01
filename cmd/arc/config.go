// Package main provides config management commands for the arc CLI,
// supporting dotted-path keys (cli.server, server.port, etc.) backed by
// ~/.arc/config.toml via the internal/config package.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	cfgpkg "github.com/sentiolabs/arc/internal/config"
	"github.com/spf13/cobra"
)

// legacyAliases maps pre-TOML key names to their current dotted equivalents.
// These are checked before the Levenshtein fallback in normalizeKey so that
// well-known old names always produce the correct "did you mean" hint.
var legacyAliases = map[string]string{
	"server_url":   "cli.server",
	"share_author": "share.author",
	"share_server": "share.server",
	"channel":      "updates.channel",
}

var recognizedKeys = []string{
	"cli.server",
	"server.port",
	"server.db_path",
	"share.author",
	"share.server",
	"updates.channel",
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage arc configuration",
	Long:  `View and modify arc configuration. Stored at ~/.arc/config.toml.`,
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration values",
	RunE:  runConfigList,
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigGet,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

var configUnsetCmd = &cobra.Command{
	Use:   "unset <key>",
	Short: "Clear a configuration value back to its default",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigUnset,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show the config file path",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := configPath
		if p == "" {
			p = cfgpkg.DefaultPath()
		}
		fmt.Println(p)
		return nil
	},
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open the config file in $EDITOR",
	RunE:  runConfigEdit,
}

func init() {
	configCmd.AddCommand(configListCmd, configGetCmd, configSetCmd, configUnsetCmd, configPathCmd, configEditCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigList(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if outputJSON {
		outputResult(cfg)
		return nil
	}
	p := configPath
	if p == "" {
		p = cfgpkg.DefaultPath()
	}
	restartSet := map[string]bool{}
	for _, k := range cfgpkg.RequiresRestart() {
		restartSet[k] = true
	}
	printRow := func(key, value string) {
		label := strings.SplitN(key, ".", 2)[1]
		tag := ""
		if restartSet[key] {
			tag = "   (requires restart)"
		}
		fmt.Printf("  %-10s = %s%s\n", label, value, tag)
	}
	fmt.Println("[cli]")
	printRow("cli.server", cfg.CLI.Server)
	fmt.Println()
	fmt.Println("[server]")
	printRow("server.port", strconv.Itoa(cfg.Server.Port))
	printRow("server.db_path", cfg.Server.DBPath)
	fmt.Println()
	fmt.Println("[share]")
	printRow("share.author", cfg.Share.Author)
	printRow("share.server", cfg.Share.Server)
	fmt.Println()
	fmt.Println("[updates]")
	printRow("updates.channel", cfg.Updates.Channel)
	fmt.Println()
	fmt.Printf("Config: %s\n", p)
	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key, err := normalizeKey(args[0])
	if err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	value := getKey(cfg, key)
	if outputJSON {
		outputResult(map[string]string{key: value})
		return nil
	}
	fmt.Println(value)
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key, err := normalizeKey(args[0])
	if err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if err := setKey(cfg, key, args[1]); err != nil {
		return err
	}
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("Set %s = %s\n", key, args[1])
	return nil
}

func runConfigUnset(cmd *cobra.Command, args []string) error {
	key, err := normalizeKey(args[0])
	if err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	def := cfgpkg.Default()
	if err := setKey(cfg, key, getKey(def, key)); err != nil {
		return err
	}
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("Unset %s (now %s)\n", key, getKey(cfg, key))
	return nil
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	p := configPath
	if p == "" {
		p = cfgpkg.DefaultPath()
	}
	// Ensure the file exists by loading once.
	if _, err := loadConfig(); err != nil {
		return err
	}
	c := exec.Command(editor, p)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("editor exited: %w", err)
	}
	// Re-validate after edit.
	if _, err := loadConfig(); err != nil {
		return fmt.Errorf("config invalid after edit: %w", err)
	}
	return nil
}

// normalizeKey canonicalizes raw to a recognized dotted key. Dashes are
// converted to underscores; casing is normalized to lower. Returns an error
// with a "did you mean" hint when the key is unrecognized.
func normalizeKey(raw string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(raw))
	k = strings.ReplaceAll(k, "-", "_")
	for _, valid := range recognizedKeys {
		if k == valid {
			return valid, nil
		}
	}
	if alias, ok := legacyAliases[k]; ok {
		return "", fmt.Errorf("unknown key: %s (did you mean %s?)", raw, alias)
	}
	return "", fmt.Errorf("unknown key: %s%s", raw, didYouMean(k))
}

// didYouMean returns a " (did you mean <key>?)" hint if the closest
// recognized key is within edit distance 4, or a fallback list otherwise.
func didYouMean(input string) string {
	best := ""
	bestDist := 1_000_000
	for _, k := range recognizedKeys {
		d := levenshtein(input, k)
		if d < bestDist {
			bestDist = d
			best = k
		}
	}
	if best != "" && bestDist <= 4 {
		return fmt.Sprintf(" (did you mean %s?)", best)
	}
	sorted := make([]string, len(recognizedKeys))
	copy(sorted, recognizedKeys)
	sort.Strings(sorted)
	return "\nValid keys: " + strings.Join(sorted, ", ")
}

// levenshtein computes the edit distance between strings a and b.
func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			m := prev[j] + 1
			if curr[j-1]+1 < m {
				m = curr[j-1] + 1
			}
			if prev[j-1]+cost < m {
				m = prev[j-1] + cost
			}
			curr[j] = m
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// getKey returns the string form of the config field for key.
func getKey(cfg *cfgpkg.Config, key string) string {
	switch key {
	case "cli.server":
		return cfg.CLI.Server
	case "server.port":
		return strconv.Itoa(cfg.Server.Port)
	case "server.db_path":
		return cfg.Server.DBPath
	case "share.author":
		return cfg.Share.Author
	case "share.server":
		return cfg.Share.Server
	case "updates.channel":
		return cfg.Updates.Channel
	}
	return ""
}

// setKey parses value and assigns it to the field identified by key.
// It then validates the whole config and returns any error.
func setKey(cfg *cfgpkg.Config, key, value string) error {
	switch key {
	case "cli.server":
		cfg.CLI.Server = value
	case "server.port":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("server.port: must be an integer")
		}
		cfg.Server.Port = n
	case "server.db_path":
		cfg.Server.DBPath = value
	case "share.author":
		cfg.Share.Author = value
	case "share.server":
		cfg.Share.Server = value
	case "updates.channel":
		cfg.Updates.Channel = value
	}
	if err := cfgpkg.Validate(cfg); err != nil {
		return err
	}
	return nil
}
