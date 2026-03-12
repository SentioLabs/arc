package main

import (
	"fmt"
	"path/filepath"

	"github.com/sentiolabs/arc/internal/project"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database management commands",
}

var dbBackupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Create a compressed backup of the database",
	Long: `Create a timestamped, gzip-compressed backup of the arc database.

The backup file is written next to the database file with the format:
  <dbfile>.<YYYYMMDD_HHMMSS>.gz

Example:
  ~/.arc/data.db.20260312_155850.gz`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")
		if dbPath == "" {
			dbPath = filepath.Join(project.DefaultArcHome(), "data.db")
		}

		result, err := sqlite.BackupDatabase(dbPath)
		if err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
		if result == nil {
			fmt.Println("Nothing to back up: database file does not exist or is empty.")
			return nil
		}

		fmt.Printf("Backup created: %s (%s → %s)\n",
			result.Path,
			formatSize(result.OriginalSize),
			formatSize(result.BackupSize),
		)
		return nil
	},
}

func init() {
	dbBackupCmd.Flags().String("db", "", "Database path (default: ~/.arc/data.db)")
	dbCmd.AddCommand(dbBackupCmd)
	rootCmd.AddCommand(dbCmd)
}

// formatSize returns a human-readable file size string.
func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
	)
	switch {
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
