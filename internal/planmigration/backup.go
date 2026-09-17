package planmigration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"github.com/sentiolabs/arc/internal/planfiles"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
)

const (
	backupDirectoryPerm = 0o700
	backupFilePerm      = 0o600
)

// BackupManifest binds one SQLite snapshot to all referenced immutable blobs.
// Extra orphan files are retained in the full root copy for operator inspection.
type BackupManifest struct {
	Version        int              `json:"version"`
	DatabaseSHA256 string           `json:"database_sha256"`
	References     []planfiles.Blob `json:"references"`
}

// Backup keeps publisher exclusion across SQLite snapshot, full root copy and
// staging verification. It never restores or modifies the source database.
func Backup(ctx context.Context, dbPath, root, destination string) (*BackupManifest, error) {
	maintenance, err := planfiles.OpenMaintenance(root)
	if err != nil {
		return nil, err
	}
	defer maintenance.Close()
	if err := maintenance.CheckDestination(destination); err != nil {
		return nil, err
	}
	s, err := sqlite.OpenPlanOperator(dbPath, false)
	if err != nil {
		return nil, err
	}
	defer s.Close()
	if err := os.Mkdir(destination, backupDirectoryPerm); err != nil {
		return nil, err
	}
	// The snapshot is a standalone SQLite file; copying a live WAL database is unsafe.
	database := filepath.Join(destination, "data.db")
	if err := s.BackupPlansDatabase(ctx, database); err != nil {
		return nil, err
	}
	if err := maintenance.CopyTo(ctx, filepath.Join(destination, "plans")); err != nil {
		return nil, err
	}
	// Read references from the captured database, never a later source DB view.
	snapshot, err := sqlite.OpenPlanOperator(database, false)
	if err != nil {
		return nil, err
	}
	references, readErr := snapshot.PlanBlobs(ctx)
	if err := errors.Join(readErr, snapshot.Close()); err != nil {
		return nil, err
	}
	// Hash the standalone database before loading it without migration.
	digest, err := hashFile(database)
	if err != nil {
		return nil, err
	}
	manifest := &BackupManifest{Version: 1, DatabaseSHA256: digest, References: references}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(destination, "manifest.json"), data, backupFilePerm); err != nil {
		return nil, err
	}
	// Verify in staging while source exclusion is still held.
	if err := VerifyBackup(ctx, destination); err != nil {
		return nil, err
	}
	return manifest, nil
}

// VerifyBackup opens the pair in staging without migration, checks database
// bytes/integrity and exact reference inventory, then verifies every blob. Use
// only a verified DB/root pair to restore; old binaries must never open a
// migrated live DB. Upgrade rollback restores the pre-upgrade pair instead.
func VerifyBackup(ctx context.Context, directory string) error {
	var manifest BackupManifest
	if err := readJSON(filepath.Join(directory, "manifest.json"), &manifest); err != nil {
		return err
	}
	if manifest.Version != 1 {
		return errors.New("unsupported backup manifest version")
	}
	maintenance, err := planfiles.OpenMaintenance(filepath.Join(directory, "plans"))
	if err != nil {
		return err
	}
	defer maintenance.Close()
	database := filepath.Join(directory, "data.db")
	// Hash the standalone database before loading it without migration.
	digest, err := hashFile(database)
	if err != nil {
		return err
	}
	if digest != manifest.DatabaseSHA256 {
		return errors.New("backup database SHA-256 mismatch")
	}
	s, err := sqlite.OpenPlanOperator(database, false)
	if err != nil {
		return err
	}
	defer s.Close()
	// A valid checksum alone does not establish SQLite structural integrity.
	var integrity string
	if err := s.DB().QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return err
	}
	if integrity != "ok" {
		return fmt.Errorf("backup database integrity: %s", integrity)
	}
	// Compare the complete inventory, including archived revisions.
	references, err := s.PlanBlobs(ctx)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(references, manifest.References) {
		return errors.New("backup reference manifest differs from database")
	}
	// Unreferenced regular content is deliberately retained; unsafe entries fail.
	reports, err := maintenance.Inspect(ctx, references)
	if err != nil {
		return err
	}
	for _, report := range reports {
		if report.Status != "ok" && report.Status != "orphan" {
			return fmt.Errorf("backup content %s: %s", report.Path, report.Status)
		}
	}
	return nil
}

// hashFile streams the database bytes without interpreting caller-selected SQL.
func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, readErr := io.Copy(hash, file)
	if err := errors.Join(readErr, file.Close()); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Inspect holds exclusive maintenance access for a stable referenced inventory.
// A server is never stopped automatically to obtain this access.
func Inspect(ctx context.Context, dbPath, root string) ([]planfiles.Inspection, error) {
	maintenance, err := planfiles.OpenMaintenance(root)
	if err != nil {
		return nil, err
	}
	defer maintenance.Close()
	s, err := sqlite.OpenPlanOperator(dbPath, false)
	if err != nil {
		return nil, err
	}
	defer s.Close()
	// Compare the complete inventory, including archived revisions.
	references, err := s.PlanBlobs(ctx)
	if err != nil {
		return nil, err
	}
	return maintenance.Inspect(ctx, references)
}

// Cleanup revalidates a selected prior dry-run report while holding exclusion.
func Cleanup(ctx context.Context, dbPath, root string, selected []string, dryRun bool) error {
	maintenance, err := planfiles.OpenMaintenance(root)
	if err != nil {
		return err
	}
	defer maintenance.Close()
	s, err := sqlite.OpenPlanOperator(dbPath, false)
	if err != nil {
		return err
	}
	defer s.Close()
	// Compare the complete inventory, including archived revisions.
	references, err := s.PlanBlobs(ctx)
	if err != nil {
		return err
	}
	return maintenance.Cleanup(ctx, references, selected, dryRun)
}
