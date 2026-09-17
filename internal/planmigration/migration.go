// Package planmigration implements explicitly selected local legacy imports and
// offline maintenance. HTTP never passes filesystem paths to this package.
package planmigration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/sentiolabs/arc/internal/planfiles"
	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
)

// Entry assigns ownership explicitly; source directories carry no authority.
type Entry struct {
	LegacyID   string `json:"legacy_id"`
	ProjectID  string `json:"project_id"`
	SourceFile string `json:"source_file"`
}

// Manifest is a set of independent imports. Duplicate IDs fail every occurrence.
type Manifest struct {
	Entries []Entry `json:"entries"`
}

// Report always names the preserved identity, including partial failures.
type Report struct {
	LegacyID string `json:"legacy_id"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
}

// ReadManifest strictly decodes the operator's explicit local selection.
func ReadManifest(path string) (Manifest, error) {
	var manifest Manifest
	err := readJSON(path, &manifest)
	if err == nil && len(manifest.Entries) == 0 {
		err = errors.New("manifest requires entries")
	}
	return manifest, err
}

// readJSON rejects unknown fields and trailing documents before any apply action.
func readJSON(path string, target any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, planfiles.MaxContentBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("expected exactly one JSON document")
	}
	return nil
}

// Migrate never initializes the DB/root in dry-run. Apply opens a publisher
// before any writes and holds its lifetime lock until every DB commit returns.
// Read-only SQLite may create transient lock/index sidecars to observe live WAL;
// it never adds durable WAL records, content, IDs or migration markers.
func Migrate(ctx context.Context, dbPath, root string, manifest Manifest, dryRun bool) ([]Report, error) {
	if len(manifest.Entries) == 0 {
		return nil, errors.New("manifest requires entries")
	}
	var publisher *planfiles.Store
	if !dryRun {
		var err error
		publisher, err = planfiles.New(root)
		if err != nil {
			return nil, err
		}
		defer publisher.Close()
	}
	s, err := sqlite.OpenPlanOperator(dbPath, !dryRun)
	if err != nil {
		return nil, err
	}
	defer s.Close()
	if publisher != nil {
		s.SetPlanPublisher(publisher)
	}
	// Pre-count IDs so no first occurrence can commit before a duplicate is discovered.
	counts := map[string]int{}
	for _, entry := range manifest.Entries {
		counts[entry.LegacyID]++
	}
	reports := make([]Report, 0, len(manifest.Entries))
	var failures []error
	for _, entry := range manifest.Entries {
		report := Report{LegacyID: entry.LegacyID}
		if counts[entry.LegacyID] > 1 {
			err = errors.New("duplicate legacy_id in manifest")
		} else {
			report.Status, err = migrateOne(ctx, s, entry, dryRun)
		}
		if err != nil {
			report.Status = "error"
			report.Error = err.Error()
			failures = append(failures, fmt.Errorf("%s: %w", entry.LegacyID, err))
		}
		reports = append(reports, report)
	}
	return reports, errors.Join(failures...)
}

// migrateOne validates ownership and sources before publication. A committed
// import can replay after its original source disappears, but changed readable
// bytes or conflicting source/project assignments never silently reimport.
func migrateOne(ctx context.Context, s *sqlite.Store, entry Entry, dryRun bool) (string, error) {
	if entry.LegacyID == "" || entry.ProjectID == "" || !filepath.IsAbs(entry.SourceFile) {
		return "", errors.New("legacy_id, project_id and absolute source_file are required")
	}
	if _, err := s.GetProject(ctx, entry.ProjectID); err != nil {
		return "", err
	}
	if _, err := s.GetPlan(ctx, entry.LegacyID); err != nil {
		return "", err
	}
	// A committed marker retains ownership even when a source file later disappears.
	marker, err := s.LegacyImportAssignment(ctx, entry.LegacyID)
	if err != nil {
		return "", err
	}
	if marker != nil && (marker.ProjectID != entry.ProjectID || marker.SourceFile != entry.SourceFile) {
		return "", storage.ErrPlanConflict
	}
	content, err := readSource(entry.SourceFile)
	if marker != nil {
		return "already_imported", checkReplaySource(marker.Content, content, err)
	}
	if err != nil {
		return "", err
	}
	// Detect ID conflicts during dry-run as well as in the commit transaction.
	var existing int
	err = s.DB().QueryRowContext(ctx, "SELECT count(*) FROM plans WHERE id=?", entry.LegacyID).Scan(&existing)
	if err != nil {
		return "", err
	}
	if existing != 0 {
		return "", storage.ErrPlanConflict
	}
	if dryRun {
		return "ready", nil
	}
	result, err := s.ImportLegacyPlan(
		ctx,
		storage.LegacyPlanImport{
			LegacyID:   entry.LegacyID,
			ProjectID:  entry.ProjectID,
			SourceFile: entry.SourceFile,
			Content:    string(content),
		},
	)
	if err != nil {
		return "", err
	}
	if result.Replay {
		return "already_imported", nil
	}
	return "imported", nil
}

// readSource bounds reads and refuses symlinks/devices before opening. The only
// path accepted here is the operator's manifest entry, never the stored path.
func readSource(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o444 == 0 {
		return nil, errors.New("source must be a readable regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	content, readErr := io.ReadAll(io.LimitReader(f, planfiles.MaxContentBytes+1))
	if err := errors.Join(readErr, f.Close()); err != nil {
		return nil, err
	}
	if len(content) > planfiles.MaxContentBytes || !utf8.Valid(content) {
		return nil, errors.New("source must be UTF-8 and at most 10 MiB")
	}
	return content, nil
}

// checkReplaySource permits lost local originals but rejects changed readable
// bytes and malformed files. The committed artifact itself is never rewritten.
func checkReplaySource(retained string, content []byte, readErr error) error {
	if errors.Is(readErr, os.ErrNotExist) {
		return nil
	}
	if readErr != nil {
		return readErr
	}
	if string(content) != retained {
		return fmt.Errorf("%w: source changed since import", storage.ErrPlanConflict)
	}
	return nil
}
