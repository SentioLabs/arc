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

	if err := store.SetProjectConfig(ctx, proj.ID, "plans.dir", "~/vault/plans"); err != nil {
		t.Fatalf("set config: %v", err)
	}
	if err := store.SetProjectConfig(ctx, proj.ID, "plans.type", "obsidian"); err != nil {
		t.Fatalf("set second key: %v", err)
	}

	got, err := store.GetProjectConfig(ctx, proj.ID)
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if got["plans.dir"] != "~/vault/plans" {
		t.Errorf("plans.dir = %q, want %q", got["plans.dir"], "~/vault/plans")
	}
	if got["plans.type"] != "obsidian" {
		t.Errorf("plans.type = %q, want %q", got["plans.type"], "obsidian")
	}

	// Setting an existing key overwrites it rather than inserting a duplicate.
	if err := store.SetProjectConfig(ctx, proj.ID, "plans.dir", "/elsewhere"); err != nil {
		t.Fatalf("overwrite config: %v", err)
	}
	got, err = store.GetProjectConfig(ctx, proj.ID)
	if err != nil {
		t.Fatalf("get config after overwrite: %v", err)
	}
	if got["plans.dir"] != "/elsewhere" {
		t.Errorf("plans.dir = %q, want %q", got["plans.dir"], "/elsewhere")
	}

	if err := store.DeleteProjectConfig(ctx, proj.ID, "plans.dir"); err != nil {
		t.Fatalf("delete config: %v", err)
	}
	got, err = store.GetProjectConfig(ctx, proj.ID)
	if err != nil {
		t.Fatalf("get config after delete: %v", err)
	}
	if _, ok := got["plans.dir"]; ok {
		t.Error("plans.dir still present after delete")
	}
	if got["plans.type"] != "obsidian" {
		t.Errorf("delete removed unrelated key: plans.type = %q", got["plans.type"])
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

	if err := store.SetProjectConfig(ctx, projA.ID, "plans.dir", "/a"); err != nil {
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
