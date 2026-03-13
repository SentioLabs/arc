package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/project"
	"github.com/spf13/cobra"
)

const (
	// pathTypeCanonical is the path type for canonical (non-symlink) paths.
	pathTypeCanonical = "canonical"
	// pathTypeSymlink is the path type for symlink paths.
	pathTypeSymlink = "symlink"
)

// legacyProjectConfig is the old per-project config format used before
// workspace paths were managed server-side.
type legacyProjectConfig struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	ProjectRoot   string `json:"project_root"`
}

// migratePathsCmd reads existing ~/.arc/projects/ configs and registers them
// as workspace paths via the server API, then cleans up each migrated config.
var migratePathsCmd = &cobra.Command{
	Use:   "migrate-paths",
	Short: "Migrate local project configs to server-side workspace paths",
	Long: `Migration that reads existing ~/.arc/projects/ configs and
registers them as workspace paths via the server API.

Both the original path and the symlink-resolved path are registered
so that lookups work regardless of how the directory is accessed.

Each successfully migrated project config directory is removed individually.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		arcHome := project.DefaultArcHome()

		configs, err := readProjectConfigs(arcHome)
		if err != nil {
			return fmt.Errorf("read project configs: %w", err)
		}

		if len(configs) == 0 {
			fmt.Println("No project configs found to migrate.")
			return nil
		}

		hostname, _ := os.Hostname()
		migrated := 0

		for _, cfg := range configs {
			absPath, resolvedPath := project.NormalizePathPair(cfg.ProjectRoot)

			if dryRun {
				if absPath != resolvedPath {
					fmt.Printf("Would migrate: %s (+ resolved: %s) → %s\n", absPath, resolvedPath, cfg.WorkspaceName)
				} else {
					fmt.Printf("Would migrate: %s → %s\n", absPath, cfg.WorkspaceName)
				}
				migrated++
				continue
			}

			c, err := getClient()
			if err != nil {
				return err
			}

			if err := registerPathPair(c, cfg.WorkspaceID, absPath, resolvedPath, hostname); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to migrate %s: %v\n", cfg.ProjectRoot, err)
				continue
			}

			if err := removeLegacyConfig(arcHome, cfg.ProjectRoot); err != nil {
				_, _ = fmt.Fprintf(os.Stderr,
					"Warning: migrated %s but failed to clean up old config: %v\n",
					cfg.ProjectRoot, err)
			}

			if absPath != resolvedPath {
				fmt.Printf("Migrated: %s (+ resolved: %s) → %s\n", absPath, resolvedPath, cfg.WorkspaceName)
			} else {
				fmt.Printf("Migrated: %s → %s\n", absPath, cfg.WorkspaceName)
			}
			migrated++
		}

		if dryRun {
			fmt.Printf("\nDry run: %d paths would be migrated.\n", migrated)
		} else {
			fmt.Printf("Migrated %d paths.\n", migrated)
		}

		return nil
	},
}

func init() {
	migratePathsCmd.Flags().Bool("dry-run", false, "Show what would be migrated without making changes")
	rootCmd.AddCommand(migratePathsCmd)
}

// registerPathPair registers both the absolute and resolved paths for a workspace.
// If both paths are the same, only one is registered. Duplicate path errors are
// silently ignored (the path is already registered).
func registerPathPair(c *client.Client, wsID, absPath, resolvedPath, hostname string) error {
	label := filepath.Base(absPath)

	// Determine path type for the absolute path
	absPathType := pathTypeCanonical
	if absPath != resolvedPath {
		absPathType = pathTypeSymlink
	}

	// Register the absolute path (what user sees / cwd reports)
	pathReq := client.CreateWorkspaceRequest{
		Path:     absPath,
		Label:    label,
		Hostname: hostname,
		PathType: absPathType,
	}
	if _, err := c.CreateWorkspace(wsID, pathReq); err != nil {
		if !isDuplicatePathError(err) {
			return fmt.Errorf("register path %s: %w", absPath, err)
		}
		// Path already exists — ensure path_type is up to date
		ensurePathType(c, wsID, absPath, absPathType)
	}

	// Register the resolved path if it differs
	if resolvedPath != absPath {
		resolvedReq := client.CreateWorkspaceRequest{
			Path:     resolvedPath,
			Label:    label + " (resolved)",
			Hostname: hostname,
			PathType: pathTypeCanonical,
		}
		if _, err := c.CreateWorkspace(wsID, resolvedReq); err != nil {
			if !isDuplicatePathError(err) {
				return fmt.Errorf("register resolved path %s: %w", resolvedPath, err)
			}
			// Path already exists — ensure path_type is up to date
			ensurePathType(c, wsID, resolvedPath, pathTypeCanonical)
		}
	}

	return nil
}

// isDuplicatePathError returns true if the error indicates the path is already registered.
func isDuplicatePathError(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "already registered") ||
		strings.Contains(err.Error(), "UNIQUE constraint") ||
		strings.Contains(err.Error(), "duplicate") ||
		strings.Contains(err.Error(), "conflict"))
}

// readLegacyConfig reads a single legacy project config matching the given path.
// Returns nil if no matching config is found.
func readLegacyConfig(arcHome, path string) (*legacyProjectConfig, error) {
	configs, err := readProjectConfigs(arcHome)
	if err != nil {
		return nil, err
	}

	// Normalize the lookup path for comparison
	absPath, resolvedPath := project.NormalizePathPair(path)

	for _, cfg := range configs {
		cfgAbs, cfgResolved := project.NormalizePathPair(cfg.ProjectRoot)
		if cfgAbs == absPath || cfgResolved == resolvedPath ||
			cfgAbs == resolvedPath || cfgResolved == absPath {
			return &cfg, nil
		}
	}
	return nil, nil //nolint:nilnil // no matching config is a valid non-error result
}

// removeLegacyConfig removes the legacy project config directory for a given path.
func removeLegacyConfig(arcHome, path string) error {
	projDir := filepath.Join(arcHome, "projects")

	entries, err := os.ReadDir(projDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	absPath, resolvedPath := project.NormalizePathPair(path)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		cfgPath := filepath.Join(projDir, entry.Name(), "config.json")
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			continue
		}

		var cfg legacyProjectConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}

		cfgAbs, cfgResolved := project.NormalizePathPair(cfg.ProjectRoot)
		if cfgAbs == absPath || cfgResolved == resolvedPath ||
			cfgAbs == resolvedPath || cfgResolved == absPath {
			return os.RemoveAll(filepath.Join(projDir, entry.Name()))
		}
	}
	return nil
}

// readProjectConfigs reads all project configs from arcHome/projects/ subdirectories.
func readProjectConfigs(arcHome string) ([]legacyProjectConfig, error) {
	projDir := filepath.Join(arcHome, "projects")

	entries, err := os.ReadDir(projDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read projects dir: %w", err)
	}

	var configs []legacyProjectConfig
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		cfgPath := filepath.Join(projDir, entry.Name(), "config.json")
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			continue // skip dirs without config.json
		}

		var cfg legacyProjectConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue // skip unparseable configs
		}

		configs = append(configs, cfg)
	}

	return configs, nil
}

// ensurePathType looks up an existing workspace path by filesystem path and updates
// its path_type if it doesn't match the desired value. This handles the case where
// paths were created before path_type was introduced (defaulting to "canonical").
func ensurePathType(c *client.Client, wsID, fsPath, desiredType string) {
	paths, err := c.ListWorkspaces(wsID)
	if err != nil {
		return
	}
	for _, p := range paths {
		if p.Path == fsPath && p.PathType != desiredType {
			_, _ = c.UpdateWorkspace(wsID, p.ID, map[string]string{
				"path_type": desiredType,
			})
			return
		}
	}
}

// backupProjectsDir renames projects/ to projects.bak/.
func backupProjectsDir(arcHome string) error {
	projDir := filepath.Join(arcHome, "projects")
	bakDir := filepath.Join(arcHome, "projects.bak")

	if _, err := os.Stat(bakDir); err == nil {
		return fmt.Errorf("backup directory %s already exists; remove it first", bakDir)
	}

	return os.Rename(projDir, bakDir)
}
