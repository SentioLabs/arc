// Package main provides config management commands for the arc CLI,
// supporting dotted-path keys (cli.server, server.port, etc.) backed by
// ~/.arc/config.toml via the internal/config package.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	cfgpkg "github.com/sentiolabs/arc/internal/config"
	"github.com/spf13/cobra"
)

// setArgsCount is the exact number of arguments required by "config set".
const setArgsCount = 2

// dottedKeyParts is the number of parts produced by splitting a dotted config key.
const dottedKeyParts = 2

// Config key names, used as the canonical dotted identifiers throughout the
// config commands.
const (
	cliServerKey      = "cli.server"
	shareAuthorKey    = "share.author"
	shareServerKey    = "share.server"
	updatesChannelKey = "updates.channel"
	plansDirKey       = "plans.dir"
	serverPortKey     = "server.port"
	serverDBPathKey   = "server.db_path"
)

// projectVar is the template variable name for a project's slug in plans.dir.
const projectVar = "project"

// cmdEdit is the cobra Use string for the "config edit" sub-command.
const cmdEdit = "edit"

// legacyAliases maps pre-TOML key names to their current dotted equivalents.
// These are checked before the Levenshtein fallback in normalizeKey so that
// well-known old names always produce the correct "did you mean" hint.
var legacyAliases = map[string]string{
	"server_url":   cliServerKey,
	"share_author": shareAuthorKey,
	"share_server": shareServerKey,
	"channel":      updatesChannelKey,
}

// recognizedKeys is the canonical list of all valid config key names.
var recognizedKeys = []string{
	cliServerKey,
	plansDirKey,
	serverPortKey,
	serverDBPathKey,
	shareAuthorKey,
	shareServerKey,
	updatesChannelKey,
}

// configCmd is the parent command for all config sub-commands.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage arc configuration",
	Long:  `View and modify arc configuration. Stored at ~/.arc/config.toml.`,
}

// configListCmd prints all configuration key=value pairs.
var configListCmd = &cobra.Command{
	Use:   cmdList,
	Short: "List all configuration values",
	RunE:  runConfigList,
}

// configGetCmd retrieves and prints a single config value by key.
var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigGet,
}

// configSetCmd writes a new value for a config key to disk.
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(setArgsCount),
	RunE:  runConfigSet,
}

// configUnsetCmd resets a config key to its compiled-in default.
var configUnsetCmd = &cobra.Command{
	Use:   "unset <key>",
	Short: "Clear a configuration value back to its default",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigUnset,
}

// configPathCmd prints the filesystem path of the active config file.
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

// configEditCmd opens the config file in $EDITOR for direct editing.
var configEditCmd = &cobra.Command{
	Use:   cmdEdit,
	Short: "Open the config file in $EDITOR",
	RunE:  runConfigEdit,
}

// resolvedFlag enables path resolution for keys that support it (plans.dir).
var resolvedFlag bool

// init registers all config sub-commands with the root command.
func init() {
	configCmd.AddCommand(configListCmd, configGetCmd, configSetCmd, configUnsetCmd, configPathCmd, configEditCmd)
	rootCmd.AddCommand(configCmd)
	configGetCmd.Flags().BoolVar(&resolvedFlag, "resolved", false,
		"resolve {vars} and ~ to an absolute path (plans.dir only)")
}

// runConfigList prints all settings grouped by TOML section.
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
		label := strings.SplitN(key, ".", dottedKeyParts)[1]
		tag := ""
		if restartSet[key] {
			tag = "   (requires restart)"
		}
		fmt.Printf("  %-10s = %s%s\n", label, value, tag)
	}
	fmt.Println("[cli]")
	printRow(cliServerKey, cfg.CLI.Server)
	fmt.Println()
	fmt.Println("[server]")
	printRow(serverPortKey, strconv.Itoa(cfg.Server.Port))
	printRow(serverDBPathKey, cfg.Server.DBPath)
	fmt.Println()
	fmt.Println("[share]")
	printRow(shareAuthorKey, cfg.Share.Author)
	printRow(shareServerKey, cfg.Share.Server)
	fmt.Println()
	fmt.Println("[updates]")
	printRow(updatesChannelKey, cfg.Updates.Channel)
	fmt.Println()
	fmt.Printf("Config: %s\n", p)
	return nil
}

// runConfigGet prints the value of a single config key.
func runConfigGet(cmd *cobra.Command, args []string) error {
	key, err := normalizeKey(args[0])
	if err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if resolvedFlag && key != plansDirKey {
		return errors.New("--resolved is only supported for plans.dir")
	}
	if resolvedFlag && key == plansDirKey {
		return runConfigGetResolved(cfg)
	}
	value := getKey(cfg, key)
	if outputJSON {
		outputResult(map[string]string{key: value})
		return nil
	}
	fmt.Println(value)
	return nil
}

// runConfigGetResolved expands plans.dir against the active project's
// template variables and current working directory, then prints the resulting
// absolute path. It hard-errors when no project can be resolved.
func runConfigGetResolved(cfg *cfgpkg.Config) error {
	c, err := getClient()
	if err != nil {
		return err
	}
	wsID, _, _, err := resolveProject()
	if err != nil {
		return err
	}
	proj, err := c.GetProject(wsID)
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}
	dir, err := cfgpkg.ExpandPlansDir(cfg.Plans.Dir, map[string]string{
		projectVar: cfgpkg.SanitizeSlug(proj.Name),
		"prefix":   proj.Prefix,
	}, cwd)
	if err != nil {
		return err
	}
	if outputJSON {
		outputResult(map[string]string{plansDirKey: dir})
		return nil
	}
	fmt.Println(dir)
	return nil
}

// runConfigSet validates and persists a new value for a config key.
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

// runConfigUnset resets a key to its default and persists the result.
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

// runConfigEdit opens the config file in $EDITOR and re-validates on save.
// Defaults to "vi" if $EDITOR is unset.
func runConfigEdit(cmd *cobra.Command, args []string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		// Fall back to vi if no EDITOR is configured.
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
	c := exec.Command(editor, p) //nolint:gosec // $EDITOR is intentionally user-controlled
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("editor exited: %w", err)
	}
	// Re-validate after edit to catch any syntax errors introduced by the user.
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
	case cliServerKey:
		return cfg.CLI.Server
	case plansDirKey:
		return cfg.Plans.Dir
	case serverPortKey:
		return strconv.Itoa(cfg.Server.Port)
	case serverDBPathKey:
		return cfg.Server.DBPath
	case shareAuthorKey:
		return cfg.Share.Author
	case shareServerKey:
		return cfg.Share.Server
	case updatesChannelKey:
		return cfg.Updates.Channel
	}
	return ""
}

// setKey parses value and assigns it to the field identified by key.
// It then validates the whole config and returns any error.
func setKey(cfg *cfgpkg.Config, key, value string) error {
	switch key {
	case cliServerKey:
		cfg.CLI.Server = value
	case plansDirKey:
		cfg.Plans.Dir = value
	case serverPortKey:
		n, err := strconv.Atoi(value)
		if err != nil {
			return errors.New("server.port: must be an integer")
		}
		cfg.Server.Port = n
	case serverDBPathKey:
		cfg.Server.DBPath = value
	case shareAuthorKey:
		cfg.Share.Author = value
	case shareServerKey:
		cfg.Share.Server = value
	case updatesChannelKey:
		cfg.Updates.Channel = value
	}
	if err := cfgpkg.Validate(cfg); err != nil {
		return err
	}
	return nil
}
