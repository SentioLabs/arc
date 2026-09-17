package planmigration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sentiolabs/arc/internal/planfiles"
	"github.com/sentiolabs/arc/internal/planmigration"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/stretchr/testify/require"
)

func fixture(t *testing.T) (store *sqlite.Store, rootPath, projectID string) {
	t.Helper()
	base := t.TempDir()
	s, err := sqlite.New(filepath.Join(base, "data.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	p := &types.Project{ID: "project", Name: "Migration fixture", Prefix: "migrate"}
	require.NoError(t, s.CreateProject(context.Background(), p))
	root := filepath.Join(base, "plans")
	publisher, err := planfiles.New(root)
	require.NoError(t, err)
	require.NoError(t, publisher.Close())
	return s, root, p.ID
}

func TestMigrationDryRunPartialResumeAndValidation(t *testing.T) {
	s, root, project := fixture(t)
	ctx := context.Background()
	content := []byte("---\r\nstatus: approved\r\n---\r\nEdited content ✓\n")
	source := filepath.Join(t.TempDir(), "draft.md")
	require.NoError(t, os.WriteFile(source, content, 0o600))
	for _, id := range []string{"plan.good", "plan.missing", "plan.unreadable", "plan.unknown"} {
		require.NoError(
			t,
			s.CreatePlan(ctx, &types.LegacyPlan{ID: id, FilePath: "/unavailable/" + id, Status: "approved"}),
		)
	}
	blocked := filepath.Join(t.TempDir(), "blocked.md")
	require.NoError(t, os.WriteFile(blocked, []byte("no access"), 0o000))
	manifest := planmigration.Manifest{Entries: []planmigration.Entry{
		{LegacyID: "plan.good", ProjectID: project, SourceFile: source},
		{LegacyID: "plan.missing", ProjectID: project, SourceFile: source + ".missing"},
		{LegacyID: "plan.unreadable", ProjectID: project, SourceFile: blocked},
		{LegacyID: "plan.unknown", ProjectID: "unknown", SourceFile: source},
	}}
	before, err := os.ReadDir(root)
	require.NoError(t, err)
	reports, err := planmigration.Migrate(ctx, s.Path(), root, manifest, true)
	require.Error(t, err)
	require.Equal(t, "ready", reports[0].Status)
	require.Equal(t, "error", reports[1].Status)
	require.Equal(t, "error", reports[2].Status)
	require.Equal(t, "error", reports[3].Status)
	after, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Len(t, after, len(before))
	var count int
	require.NoError(t, s.DB().QueryRow("SELECT count(*) FROM plans").Scan(&count))
	require.Zero(t, count)
	require.NoError(t, s.DB().QueryRow("SELECT count(*) FROM plan_idempotency").Scan(&count))
	require.Zero(t, count)
	reports, err = planmigration.Migrate(ctx, s.Path(), root, manifest, false)
	require.Error(t, err)
	require.Equal(t, "imported", reports[0].Status)
	require.NoError(t, os.WriteFile(source+".missing", []byte("recovered"), 0o600))
	require.NoError(t, os.Chmod(blocked, 0o600))
	manifest.Entries = manifest.Entries[:3]
	reports, err = planmigration.Migrate(ctx, s.Path(), root, manifest, false)
	require.NoError(t, err)
	require.Equal(t, "already_imported", reports[0].Status)
	require.Equal(t, "imported", reports[1].Status)
	require.Equal(t, "imported", reports[2].Status)
	got, err := os.ReadFile(source)
	require.NoError(t, err)
	require.Equal(t, content, got)
	require.NoError(t, s.DB().QueryRow("SELECT count(*) FROM plans").Scan(&count))
	require.Equal(t, 3, count)
	duplicate := planmigration.Manifest{Entries: []planmigration.Entry{manifest.Entries[0], manifest.Entries[0]}}
	reports, err = planmigration.Migrate(ctx, s.Path(), root, duplicate, false)
	require.Error(t, err)
	require.Equal(t, "error", reports[0].Status)
	require.Equal(t, "error", reports[1].Status)
	require.NoError(t, os.WriteFile(source, []byte("tampered"), 0o600))
	_, err = planmigration.Migrate(ctx, s.Path(), root, planmigration.Manifest{Entries: manifest.Entries[:1]}, false)
	require.Error(t, err)
}

func TestBackupStagingVerificationAndBlobLoss(t *testing.T) {
	s, root, project := fixture(t)
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "source.md")
	require.NoError(t, os.WriteFile(source, []byte("retained exact\r\n"), 0o600))
	require.NoError(t, s.CreatePlan(ctx, &types.LegacyPlan{ID: "plan.backup", FilePath: source, Status: "approved"}))
	_, err := planmigration.Migrate(
		ctx,
		s.Path(),
		root,
		planmigration.Manifest{Entries: []planmigration.Entry{
			{LegacyID: "plan.backup", ProjectID: project, SourceFile: source},
		}},
		false,
	)
	require.NoError(t, err)
	target := filepath.Join(t.TempDir(), "backup")
	manifest, err := planmigration.Backup(ctx, s.Path(), root, target)
	require.NoError(t, err)
	require.Len(t, manifest.References, 1)
	require.NoError(t, planmigration.VerifyBackup(ctx, target))
	restored, err := sqlite.OpenPlanOperator(filepath.Join(target, "data.db"), false)
	require.NoError(t, err)
	inventory, err := restored.ListLegacyPlans(ctx, 50, 0)
	require.NoError(t, err)
	require.Len(t, inventory, 1)
	require.Equal(t, "approved", inventory[0].Status)
	// Prove the captured pair stands alone after loss in the original root.
	require.NoError(t, os.Remove(filepath.Join(root, manifest.References[0].RelativePath)))
	restoredPublisher, err := planfiles.New(filepath.Join(target, "plans"))
	require.NoError(t, err)
	restored.SetPlanPublisher(restoredPublisher)
	revision, err := restored.ReadPlanRevision(ctx, project, "plan.backup", 1)
	require.NoError(t, err)
	require.Equal(t, "retained exact\r\n", revision.Content)
	require.Equal(t, "draft", revision.ReviewStatus)
	require.NoError(t, restoredPublisher.Close())
	require.NoError(t, restored.Close())
	require.NoError(t, os.Remove(filepath.Join(target, "plans", manifest.References[0].RelativePath)))
	require.Error(t, planmigration.VerifyBackup(ctx, target))
	_, err = planmigration.Backup(ctx, s.Path(), root, filepath.Join(root, "nested"))
	require.Error(t, err)
	_, err = os.Stat(filepath.Join(root, "nested"))
	require.ErrorIs(t, err, os.ErrNotExist)
	publisher, err := planfiles.New(root)
	require.NoError(t, err)
	defer publisher.Close()
	_, err = planmigration.Backup(ctx, s.Path(), root, filepath.Join(t.TempDir(), "busy"))
	require.ErrorIs(t, err, planfiles.ErrBusy)
}

func TestDryRunPreservesDurableFilesAndReadsCommittedWAL(t *testing.T) {
	for _, closed := range []bool{false, true} {
		t.Run(map[bool]string{false: "active-wal", true: "closed-wal"}[closed], func(t *testing.T) {
			s, root, project := fixture(t)
			source := filepath.Join(t.TempDir(), "source.md")
			require.NoError(t, os.WriteFile(source, []byte("source"), 0o600))
			require.NoError(
				t,
				s.CreatePlan(t.Context(), &types.LegacyPlan{
					ID: "plan.wal", FilePath: "/not/source", Status: "approved",
				}),
			)
			database := s.Path()
			if closed {
				require.NoError(t, s.Close())
			}
			before, err := os.ReadFile(database)
			require.NoError(t, err)
			walBefore, walErr := os.ReadFile(database + "-wal")
			rootBefore, err := os.ReadDir(root)
			require.NoError(t, err)
			reports, err := planmigration.Migrate(t.Context(), database, root,
				planmigration.Manifest{Entries: []planmigration.Entry{
					{LegacyID: "plan.wal", ProjectID: project, SourceFile: source},
				}}, true)
			require.NoError(t, err)
			require.Equal(t, "ready", reports[0].Status)
			after, err := os.ReadFile(database)
			require.NoError(t, err)
			require.Equal(t, before, after)
			walAfter, err := os.ReadFile(database + "-wal")
			if walErr == nil {
				require.NoError(t, err)
				require.Equal(t, walBefore, walAfter)
			} else {
				require.Empty(t, walAfter)
			}
			rootAfter, err := os.ReadDir(root)
			require.NoError(t, err)
			require.Equal(t, rootBefore, rootAfter)
			// SQLite may create empty WAL/shared-index coordination sidecars for a
			// read-only connection; these contain no new durable records or content.
			t.Logf(
				"read-only SQLite coordination: closed=%v wal_before=%d wal_after=%d",
				closed,
				len(walBefore),
				len(walAfter),
			)
			reader, err := sqlite.OpenPlanOperator(database, false)
			require.NoError(t, err)
			defer reader.Close()
			for _, table := range []string{"plans", "plan_revisions", "plan_idempotency"} {
				var count int
				require.NoError(t, reader.DB().QueryRow("SELECT count(*) FROM "+table).Scan(&count))
				require.Zero(t, count)
			}
		})
	}
}

func TestMigrationRejectsInvalidSourceContent(t *testing.T) {
	for _, content := range [][]byte{{0xff}, make([]byte, planfiles.MaxContentBytes+1)} {
		s, root, project := fixture(t)
		require.NoError(
			t,
			s.CreatePlan(t.Context(), &types.LegacyPlan{ID: "plan.invalid", FilePath: "/unused", Status: "approved"}),
		)
		source := filepath.Join(t.TempDir(), "source")
		require.NoError(t, os.WriteFile(source, content, 0o600))
		reports, err := planmigration.Migrate(t.Context(), s.Path(), root,
			planmigration.Manifest{Entries: []planmigration.Entry{
				{LegacyID: "plan.invalid", ProjectID: project, SourceFile: source},
			}}, true)
		require.Error(t, err)
		require.Equal(t, "error", reports[0].Status)
	}
}

func TestMigrationReplayValidatesReadableSource(t *testing.T) {
	s, root, project := fixture(t)
	source := filepath.Join(t.TempDir(), "source.md")
	require.NoError(t, os.WriteFile(source, []byte("original"), 0o600))
	require.NoError(
		t,
		s.CreatePlan(t.Context(), &types.LegacyPlan{ID: "plan.replay", FilePath: "/unused", Status: "approved"}),
	)
	manifest := planmigration.Manifest{Entries: []planmigration.Entry{
		{LegacyID: "plan.replay", ProjectID: project, SourceFile: source},
	}}
	_, err := planmigration.Migrate(t.Context(), s.Path(), root, manifest, false)
	require.NoError(t, err)
	require.NoError(t, os.Remove(source))
	reports, err := planmigration.Migrate(t.Context(), s.Path(), root, manifest, false)
	require.NoError(t, err)
	require.Equal(t, "already_imported", reports[0].Status)
	require.NoError(t, os.WriteFile(source, []byte{0xff}, 0o600))
	_, err = planmigration.Migrate(t.Context(), s.Path(), root, manifest, true)
	require.Error(t, err)
}
