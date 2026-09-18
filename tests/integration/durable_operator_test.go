//go:build integration

package integration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sentiolabs/arc/internal/planfiles"
	"github.com/sentiolabs/arc/internal/planmigration"
	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/stretchr/testify/require"
)

func (f *durableFixture) operator(args ...string) string {
	f.t.Helper()
	return f.cli(append([]string{"server", "plans"}, args...)...)
}

func TestDurableOperatorMigrationRecovery(t *testing.T) {
	f := newDurableFixture(t)
	f.stop()
	// Seed legacy inventory through the retained storage interface. All subsequent
	// import, inspection, backup and cleanup use the actual CLI/operator commands.
	store, err := sqlite.New(f.db)
	require.NoError(t, err)
	sources := map[string]string{
		"plan.edited":  f.file("approved-but-edited.md", "# Edited after old approval\r\n"),
		"plan.valid":   f.file("valid.md", "# Valid legacy draft\n"),
		"plan.missing": filepath.Join(f.dir, "missing.md"),
		// A directory is unreadable as Markdown on every platform, even under root.
		"plan.unreadable": f.dir,
	}
	manifest := planmigration.Manifest{}
	for id, source := range sources {
		status := "draft"
		if id == "plan.edited" {
			status = "approved"
		}
		require.NoError(t, store.CreatePlan(t.Context(), &types.LegacyPlan{ID: id, FilePath: source, Status: status}))
		manifest.Entries = append(manifest.Entries, planmigration.Entry{LegacyID: id, ProjectID: f.project, SourceFile: source})
	}
	anchor := &types.PlanCommentAnchor{LineStart: 1, LineEnd: 1, QuotedText: "Original approved text", Occurrence: 0}
	require.NoError(t, store.CreatePlanComment(t.Context(), &types.PlanComment{ID: "comment.preserved", PlanID: "plan.edited", Content: "Original review", Anchor: anchor}))
	require.NoError(t, store.Close())
	manifestPath := f.file("migration.json", manifest)
	for step, mode := range []string{"--dry-run", "--apply", "--apply"} {
		out, commandErr := f.run("", "server", "plans", "migrate", "--manifest", manifestPath, mode)
		require.Error(t, commandErr, "partial errors must return nonzero")
		// Cobra appends usage/error text after the JSON report on combined output.
		var reports []planmigration.Report
		require.NoError(t, json.NewDecoder(strings.NewReader(out)).Decode(&reports), out)
		require.Len(t, reports, 4)
		for _, report := range reports {
			if report.LegacyID == "plan.missing" || report.LegacyID == "plan.unreadable" {
				require.Equal(t, "error", report.Status)
				require.NotEmpty(t, report.Error)
			} else {
				require.Equal(t, []string{"ready", "imported", "already_imported"}[step], report.Status)
			}
		}
	}
	for id, content := range map[string]string{"plan.edited": "# Edited after old approval\r\n", "plan.valid": "# Valid legacy draft\n"} {
		actual, readErr := os.ReadFile(sources[id])
		require.NoError(t, readErr)
		require.Equal(t, content, string(actual))
	}
	store, err = sqlite.OpenPlanOperator(f.db, false)
	require.NoError(t, err)
	var legacyStatus string
	require.NoError(t, store.DB().QueryRow("SELECT legacy_status_unverified FROM plans WHERE id=?", "plan.edited").Scan(&legacyStatus))
	require.Equal(t, "approved", legacyStatus)
	inventory, err := store.ListLegacyPlans(t.Context(), 50, 0)
	require.NoError(t, err)
	require.Len(t, inventory, 4)
	comments, err := store.ListRevisionComments(t.Context(), f.project, "plan.edited", 1, true, 50, 0)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	require.Equal(t, "comment.preserved", comments[0].ID)
	require.Nil(t, comments[0].Revision)
	require.Equal(t, anchor, comments[0].Anchor)
	require.NoError(t, store.Close())
	orphan := filepath.Join(f.root, "unreferenced.md")
	require.NoError(t, os.WriteFile(orphan, []byte("orphan bytes"), 0o600))
	var inspection struct {
		DryRun bool                   `json:"dry_run"`
		Files  []planfiles.Inspection `json:"files"`
	}
	decodeDurable(t, f.operator("integrity"), &inspection)
	require.True(t, inspection.DryRun)
	var selected []planfiles.Inspection
	for _, file := range inspection.Files {
		if file.Status == "orphan" {
			selected = append(selected, file)
		} else {
			require.Equal(t, "ok", file.Status)
		}
	}
	require.Len(t, selected, 1)
	report := f.file("orphan-selection.json", map[string]any{"dry_run": true, "files": selected})
	f.operator("cleanup", "--report", report, "--dry-run")
	require.FileExists(t, orphan)
	backup := filepath.Join(f.home, "paired-backup")
	f.operator("backup", "--output", backup)
	f.operator("verify-backup", "--directory", backup)
	require.FileExists(t, filepath.Join(backup, "plans", "unreferenced.md"))
	f.operator("cleanup", "--report", report, "--apply")
	require.NoFileExists(t, orphan)
	// Restore both halves to new paths; do not mutate or open the backup itself.
	restored := filepath.Join(f.home, "restored")
	require.NoError(t, os.CopyFS(restored, os.DirFS(backup)))
	f.operator("verify-backup", "--directory", restored)
	f.db = filepath.Join(restored, "data.db")
	f.root = filepath.Join(restored, "plans")
	rewriteDurableConfig(t, f)
	f.operator("integrity")
	f.start(arcBinary)
	var shown struct {
		Plan     types.Plan                    `json:"plan"`
		Revision types.PlanRevisionWithContent `json:"revision"`
	}
	decodeDurable(t, f.cli("plan", "show", "plan.edited", "--revision", "1", "--json"), &shown)
	require.Equal(t, "plan.edited", shown.Plan.ID)
	require.Equal(t, "draft", shown.Revision.ReviewStatus)
	require.Equal(t, "# Edited after old approval\r\n", shown.Revision.Content)
	var retained []types.PlanComment
	decodeDurable(t, f.cli("plan", "comments", "plan.edited", "--revision", "1", "--json"), &retained)
	require.Len(t, retained, 1)
	require.Equal(t, "comment.preserved", retained[0].ID)
	require.Nil(t, retained[0].Revision)
	require.Equal(t, anchor, retained[0].Anchor)
	// Upgrade rejects a real existing submitted path without reading/changing it.
	body := durableAPI(t, "POST", "/api/v1/plans", "", map[string]string{"file_path": sources["plan.valid"]}, 400)
	require.Contains(t, string(body), "upgrade")
	original, err := os.ReadFile(sources["plan.valid"])
	require.NoError(t, err)
	require.Equal(t, "# Valid legacy draft\n", string(original))
	f.stop()
	t.Logf("preserved plans/comments, pending sources, paired restore and orphan selection: %s", f.project)
}

func rewriteDurableConfig(t *testing.T, f *durableFixture) {
	t.Helper()
	require.NoError(t, os.WriteFile(f.config, fmt.Appendf(nil, "[server]\ndb_path = %q\nplans_dir = %q\n", f.db, f.root), 0o600))
}

// This platform-specific companion runs explicitly with a retained immutable
// pre-upgrade binary. Docker still runs the portable operator recovery fixture.
func TestDurablePreupgradeRollback(t *testing.T) {
	old := os.Getenv("ARC_PREUPGRADE_BINARY")
	if old == "" {
		t.Skip("set ARC_PREUPGRADE_BINARY for the separate pre-upgrade rollback exercise")
	}
	require.True(t, filepath.IsAbs(old))
	f := newDurableFixtureBinary(t, old)
	source := f.file("preupgrade.md", "# Pre-upgrade approved bytes\n")
	var legacy types.LegacyPlan
	decodeDurable(t, string(durableAPI(t, "POST", "/api/v1/plans", "", map[string]string{"file_path": source}, 201)), &legacy)
	durableAPI(t, "PATCH", "/api/v1/plans/"+legacy.ID+"/status", "", map[string]string{"status": "approved"}, 200)
	var comment types.PlanComment
	decodeDurable(t, string(durableAPI(t, "POST", "/api/v1/plans/"+legacy.ID+"/comments", "", map[string]string{"content": "Pre-upgrade discussion"}, 201)), &comment)
	f.stop()
	// Take a SQLite-consistent PRE-upgrade snapshot without invoking new schema
	// initialization. Keep a full root alongside it, then upgrade the disposable DB.
	require.NoError(t, os.MkdirAll(f.root, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(f.root, "preupgrade-marker.md"), []byte("original root"), 0o600))
	pair := filepath.Join(f.home, "preupgrade-pair")
	require.NoError(t, os.Mkdir(pair, 0o700))
	db, err := sql.Open("sqlite", f.db)
	require.NoError(t, err)
	_, err = db.Exec("VACUUM INTO ?", filepath.Join(pair, "data.db"))
	require.NoError(t, err)
	require.NoError(t, db.Close())
	require.NoError(t, os.CopyFS(filepath.Join(pair, "plans"), os.DirFS(f.root)))
	originalDB, err := os.ReadFile(filepath.Join(pair, "data.db"))
	require.NoError(t, err)
	f.start(arcBinary)
	f.stop()
	manifest := f.file("upgrade-import.json", planmigration.Manifest{Entries: []planmigration.Entry{{LegacyID: legacy.ID, ProjectID: f.project, SourceFile: source}}})
	f.operator("migrate", "--manifest", manifest, "--dry-run")
	f.operator("migrate", "--manifest", manifest, "--apply")
	f.operator("integrity")
	f.start(arcBinary)
	var imported struct {
		Revision types.PlanRevisionWithContent `json:"revision"`
	}
	decodeDurable(t, f.cli("plan", "show", legacy.ID, "--revision", "1", "--json"), &imported)
	require.Equal(t, "draft", imported.Revision.ReviewStatus)
	// Export post-upgrade content before rollback; the old DB cannot retain it.
	changed := f.file("post-upgrade.md", "# Post-upgrade content to preserve separately\n")
	f.cli("plan", "update", legacy.ID, changed, "--expected-revision", "1", "--idempotency-key", "post-upgrade", "--json")
	exported := filepath.Join(f.dir, "post-upgrade-export.md")
	f.cli("plan", "export", legacy.ID, "--revision", "2", "--output", exported)
	f.stop()
	rollback := filepath.Join(f.home, "rollback")
	require.NoError(t, os.CopyFS(rollback, os.DirFS(pair)))
	f.db = filepath.Join(rollback, "data.db")
	f.root = filepath.Join(rollback, "plans")
	rewriteDurableConfig(t, f)
	db, err = sql.Open("sqlite", f.db)
	require.NoError(t, err)
	var integrity string
	require.NoError(t, db.QueryRow("PRAGMA integrity_check").Scan(&integrity))
	require.Equal(t, "ok", integrity)
	require.NoError(t, db.Close())
	marker, err := os.ReadFile(filepath.Join(f.root, "preupgrade-marker.md"))
	require.NoError(t, err)
	require.Equal(t, "original root", string(marker))
	f.start(old)
	var restored types.LegacyPlanWithContent
	decodeDurable(t, string(durableAPI(t, "GET", "/api/v1/plans/"+legacy.ID, "", nil, 200)), &restored)
	require.Equal(t, legacy.ID, restored.ID)
	require.Equal(t, "approved", restored.Status)
	require.Equal(t, "# Pre-upgrade approved bytes\n", restored.Content)
	var comments []types.PlanComment
	decodeDurable(t, string(durableAPI(t, "GET", "/api/v1/plans/"+legacy.ID+"/comments", "", nil, 200)), &comments)
	require.Len(t, comments, 1)
	require.Equal(t, comment.ID, comments[0].ID)
	f.stop()
	stillOriginal, err := os.ReadFile(filepath.Join(pair, "data.db"))
	require.NoError(t, err)
	require.Equal(t, originalDB, stillOriginal)
	retained, err := os.ReadFile(exported)
	require.NoError(t, err)
	require.Equal(t, "# Post-upgrade content to preserve separately\n", string(retained))
	t.Logf("old binary %s: pre-upgrade backup → new schema/import/export → restored old pair → old server readback PASS", old)
}
