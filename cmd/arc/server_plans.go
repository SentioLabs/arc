package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/sentiolabs/arc/internal/planfiles"
	"github.com/sentiolabs/arc/internal/planmigration"
	"github.com/spf13/cobra"
)

// newServerPlansCommand keeps local operator commands separate from HTTP plan
// clients. Output is always JSON so partial results can be retained and selected.
// Every command resolves server settings when invoked, avoiding a stale root.
// Explicit apply flags prevent a pasted preview command from mutating metadata.
// Backup and verification operate on new staging directories only.
// The commands never acquire a network client or resolve a workspace.
// Legacy approval text is preserved as provenance and never grants approval.
// The operator chooses project ownership; filesystem layout is not authority.
func newServerPlansCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   projectPlansCmdName,
		Short: "Migrate, inspect and back up server-owned plans locally",
		Long: `Local operator tools using server.db_path and server.plans_dir from --config.
These commands never use client plans.dir, stop a daemon, or restore live data.
Reports are JSON, including partial migration failures (nonzero exit).
Read-only SQLite opens can create transient WAL/shared-index lock sidecars;
dry-run never changes database records, retained content, IDs or counters.

Offline cleanup and backup require all publishers to be stopped. The upgraded
server holds a kernel exclusion lock through publication and database commit.
First run integrity and retain its dry-run JSON report; select orphan entries
in that report before cleanup --report FILE --dry-run, then --apply.

Backup creates a SQLite-consistent data.db, full plans/ tree, and manifest.json
in a new staging directory and verifies the pair. Run verify-backup again before
restoring BOTH database and root while the server is stopped. Rollback requires
the pre-upgrade DB/root pair; never run an old binary against the migrated DB.`,
	}
	migrate := &cobra.Command{
		Use:   "migrate",
		Short: "Import explicit legacy mappings; preserves IDs and unverified provenance",
		RunE:  runPlansMigrate,
		Args:  cobra.NoArgs,
	}
	migrate.Flags().String("manifest", "", "JSON entries: legacy_id, project_id, absolute source_file")
	addPlansModeFlags(migrate)
	integrity := &cobra.Command{
		Use:   "integrity",
		Short: "Inspect referenced hashes, missing content, symlinks and orphans offline",
		RunE:  runPlansIntegrity,
		Args:  cobra.NoArgs,
	}
	cleanup := &cobra.Command{
		Use:   "cleanup",
		Short: "Delete selected regular orphans from a prior dry-run report offline",
		RunE:  runPlansCleanup,
		Args:  cobra.NoArgs,
	}
	cleanup.Flags().String("report", "", "Integrity dry-run JSON report with selected orphan entries")
	addPlansModeFlags(cleanup)
	backup := &cobra.Command{
		Use:   "backup",
		Short: "Capture and verify a consistent database and complete blob root offline",
		RunE:  runPlansBackup,
		Args:  cobra.NoArgs,
	}
	backup.Flags().String("output", "", "New backup directory outside the plan root; parent must exist")
	verify := &cobra.Command{
		Use:   "verify-backup",
		Short: "Verify a staged DB/root pair against its backup manifest",
		RunE:  runPlansVerifyBackup,
		Args:  cobra.NoArgs,
	}
	verify.Flags().String("directory", "", "Staging directory containing data.db, plans/ and manifest.json")
	cmd.AddCommand(migrate, integrity, cleanup, backup, verify)
	return cmd
}

// Require operators to distinguish preview from writes; there is no implicit apply.
func addPlansModeFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("dry-run", false, "Validate and report without durable writes")
	cmd.Flags().Bool("apply", false, "Explicitly apply the selected operation")
}

// Reject both ambiguous modes before opening a database or filesystem capability.
func plansDryRun(cmd *cobra.Command) (bool, error) {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	apply, _ := cmd.Flags().GetBool("apply")
	if dryRun == apply {
		return false, errors.New("select exactly one of --dry-run or --apply")
	}
	return dryRun, nil
}

// plansOperatorPaths deliberately reads only the server configuration section.
func plansOperatorPaths() (dbPath, plansRoot string, err error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", "", err
	}
	root, err := cfg.Server.ResolvedPlansDir()
	if err != nil {
		return "", "", err
	}
	return cfg.Server.ResolvedDBPath(), root, nil
}

// Preserve partial operation results even when the process will exit unsuccessfully.
func plansJSON(cmd *cobra.Command, value any, operationErr error) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return errors.Join(operationErr, encoder.Encode(value))
}

// Load the explicit manifest locally; no HTTP path registration is performed.
func runPlansMigrate(cmd *cobra.Command, _ []string) error {
	dryRun, err := plansDryRun(cmd)
	if err != nil {
		return err
	}
	path, _ := cmd.Flags().GetString("manifest")
	if path == "" {
		return errors.New("--manifest is required")
	}
	manifest, err := planmigration.ReadManifest(path)
	if err != nil {
		return err
	}
	dbPath, root, err := plansOperatorPaths()
	if err != nil {
		return err
	}
	reports, err := planmigration.Migrate(cmd.Context(), dbPath, root, manifest, dryRun)
	return plansJSON(cmd, reports, err)
}

// plansInspectionReport is also the explicit cleanup selection artifact. Remove
// any orphan entries that should be retained before applying the report.
type plansInspectionReport struct {
	DryRun bool                   `json:"dry_run"`
	Files  []planfiles.Inspection `json:"files"`
}

// Produce a selection artifact and a nonzero status for damage or unsafe files.
func runPlansIntegrity(cmd *cobra.Command, _ []string) error {
	dbPath, root, err := plansOperatorPaths()
	if err != nil {
		return err
	}
	reports, err := planmigration.Inspect(cmd.Context(), dbPath, root)
	if err == nil {
		for _, r := range reports {
			if r.Status != "ok" && r.Status != "orphan" {
				err = errors.New("plan integrity inspection found damaged or unsafe content")
				break
			}
		}
	}
	return plansJSON(cmd, plansInspectionReport{DryRun: true, Files: reports}, err)
}

// Revalidate selected regular orphans under the same exclusion as deletion.
func runPlansCleanup(cmd *cobra.Command, _ []string) error {
	dryRun, err := plansDryRun(cmd)
	if err != nil {
		return err
	}
	path, _ := cmd.Flags().GetString("report")
	if path == "" {
		return errors.New("--report is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var report plansInspectionReport
	if err := json.Unmarshal(data, &report); err != nil {
		return err
	}
	if !report.DryRun {
		return errors.New("cleanup requires a prior dry-run report")
	}
	selected := make([]string, 0)
	for _, file := range report.Files {
		if file.Status == "orphan" {
			selected = append(selected, file.Path)
		}
	}
	dbPath, root, err := plansOperatorPaths()
	if err != nil {
		return err
	}
	err = planmigration.Cleanup(cmd.Context(), dbPath, root, selected, dryRun)
	return plansJSON(cmd, map[string]any{"dry_run": dryRun, "selected": selected, "success": err == nil}, err)
}

// Create a new staging pair without replacing any existing destination.
func runPlansBackup(cmd *cobra.Command, _ []string) error {
	destination, _ := cmd.Flags().GetString("output")
	if destination == "" {
		return errors.New("--output is required")
	}
	dbPath, root, err := plansOperatorPaths()
	if err != nil {
		return err
	}
	manifest, err := planmigration.Backup(cmd.Context(), dbPath, root, destination)
	return plansJSON(cmd, manifest, err)
}

// Verification never installs a backup or opens it with schema migrations.
func runPlansVerifyBackup(cmd *cobra.Command, _ []string) error {
	directory, _ := cmd.Flags().GetString("directory")
	if directory == "" {
		return errors.New("--directory is required")
	}
	err := planmigration.VerifyBackup(cmd.Context(), directory)
	status := "verified"
	if err != nil {
		status = fmt.Sprintf("verification failed: %v", err)
	}
	return plansJSON(cmd, map[string]string{"directory": directory, "result": status}, err)
}
