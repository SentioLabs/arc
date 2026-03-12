package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/project"
	"github.com/spf13/cobra"
)

// legacyProjectConfig is the old per-project config format used before
// workspace paths were managed server-side.
type legacyProjectConfig struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	ProjectRoot   string `json:"project_root"`
}

// migratePathsCmd reads existing ~/.arc/projects/ configs and registers them
// as workspace paths via the server API, then backs up the old configs.
var migratePathsCmd = &cobra.Command{
	Use:   "migrate-paths",
	Short: "Migrate local project configs to server-side workspace paths",
	Long: `One-time migration that reads existing ~/.arc/projects/ configs and
registers them as workspace paths via the server API.

After migration, the projects/ directory is renamed to projects.bak/.`,
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
			if dryRun {
				fmt.Printf("Would migrate: %s → %s\n", cfg.ProjectRoot, cfg.WorkspaceName)
				migrated++
				continue
			}

			c, err := getClient()
			if err != nil {
				return err
			}

			pathReq := client.CreateWorkspacePathRequest{
				Path:     cfg.ProjectRoot,
				Label:    filepath.Base(cfg.ProjectRoot),
				Hostname: hostname,
			}

			if _, err := c.CreateWorkspacePath(cfg.WorkspaceID, pathReq); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to migrate %s: %v\n", cfg.ProjectRoot, err)
				continue
			}

			fmt.Printf("Migrated: %s → %s\n", cfg.ProjectRoot, cfg.WorkspaceName)
			migrated++
		}

		if !dryRun && migrated > 0 {
			if err := backupProjectsDir(arcHome); err != nil {
				return fmt.Errorf("backup projects dir: %w", err)
			}
		}

		if dryRun {
			fmt.Printf("\nDry run: %d paths would be migrated.\n", migrated)
		} else {
			fmt.Printf("Migrated %d paths. Original configs backed up to %s\n",
				migrated, filepath.Join(arcHome, "projects.bak")+"/")
		}

		return nil
	},
}

func init() {
	migratePathsCmd.Flags().Bool("dry-run", false, "Show what would be migrated without making changes")
	rootCmd.AddCommand(migratePathsCmd)
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

// backupProjectsDir renames projects/ to projects.bak/.
func backupProjectsDir(arcHome string) error {
	projDir := filepath.Join(arcHome, "projects")
	bakDir := filepath.Join(arcHome, "projects.bak")

	if _, err := os.Stat(bakDir); err == nil {
		return fmt.Errorf("backup directory %s already exists; remove it first", bakDir)
	}

	return os.Rename(projDir, bakDir)
}
