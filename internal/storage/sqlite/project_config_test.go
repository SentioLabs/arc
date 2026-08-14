package sqlite_test

import (
	"context"
	"testing"

	"github.com/sentiolabs/arc/internal/types"
)

func TestProjectConfigRoundTrip(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	if err := store.SetProjectConfig(ctx, proj.ID, "docs.path", "~/vault/plans"); err != nil {
		t.Fatalf("set config: %v", err)
	}
	if err := store.SetProjectConfig(ctx, proj.ID, "docs.type", "obsidian"); err != nil {
		t.Fatalf("set second key: %v", err)
	}

	got, err := store.GetProjectConfig(ctx, proj.ID)
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if got["docs.path"] != "~/vault/plans" {
		t.Errorf("docs.path = %q, want %q", got["docs.path"], "~/vault/plans")
	}
	if got["docs.type"] != "obsidian" {
		t.Errorf("docs.type = %q, want %q", got["docs.type"], "obsidian")
	}

	// Setting an existing key overwrites it rather than inserting a duplicate.
	if err := store.SetProjectConfig(ctx, proj.ID, "docs.path", "/elsewhere"); err != nil {
		t.Fatalf("overwrite config: %v", err)
	}
	got, err = store.GetProjectConfig(ctx, proj.ID)
	if err != nil {
		t.Fatalf("get config after overwrite: %v", err)
	}
	if got["docs.path"] != "/elsewhere" {
		t.Errorf("docs.path = %q, want %q", got["docs.path"], "/elsewhere")
	}

	if err := store.DeleteProjectConfig(ctx, proj.ID, "docs.path"); err != nil {
		t.Fatalf("delete config: %v", err)
	}
	got, err = store.GetProjectConfig(ctx, proj.ID)
	if err != nil {
		t.Fatalf("get config after delete: %v", err)
	}
	if _, ok := got["docs.path"]; ok {
		t.Error("docs.path still present after delete")
	}
	if got["docs.type"] != "obsidian" {
		t.Errorf("delete removed unrelated key: docs.type = %q", got["docs.type"])
	}
}

func TestGetProjectConfigEmpty(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	got, err := store.GetProjectConfig(ctx, proj.ID)
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestProjectConfigIsolatedPerProject(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	projA := setupTestProject(t, store)

	projB := &types.Project{Name: "Other Project", Prefix: "other"}
	if err := store.CreateProject(ctx, projB); err != nil {
		t.Fatalf("create second project: %v", err)
	}

	if err := store.SetProjectConfig(ctx, projA.ID, "docs.path", "/a"); err != nil {
		t.Fatalf("set config on project A: %v", err)
	}

	got, err := store.GetProjectConfig(ctx, projB.ID)
	if err != nil {
		t.Fatalf("get config on project B: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("project B sees project A's config: %v", got)
	}
}
