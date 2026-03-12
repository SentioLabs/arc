package sqlite_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sentiolabs/arc/internal/types"
)

func TestCreateWorkspacePath(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

	wp := &types.WorkspacePath{
		WorkspaceID: ws.ID,
		Path:        "/home/user/projects/myapp",
		Label:       "main",
		Hostname:    "dev-machine",
		GitRemote:   "git@github.com:org/repo.git",
	}

	err := store.CreateWorkspacePath(ctx, wp)
	if err != nil {
		t.Fatalf("CreateWorkspacePath failed: %v", err)
	}

	// ID should be generated
	if wp.ID == "" {
		t.Error("expected ID to be generated")
	}

	// Retrieve and verify
	got, err := store.GetWorkspacePath(ctx, wp.ID)
	if err != nil {
		t.Fatalf("GetWorkspacePath failed: %v", err)
	}

	if got.WorkspaceID != ws.ID {
		t.Errorf("WorkspaceID = %q, want %q", got.WorkspaceID, ws.ID)
	}
	if got.Path != "/home/user/projects/myapp" {
		t.Errorf("Path = %q, want %q", got.Path, "/home/user/projects/myapp")
	}
	if got.Label != "main" {
		t.Errorf("Label = %q, want %q", got.Label, "main")
	}
	if got.Hostname != "dev-machine" {
		t.Errorf("Hostname = %q, want %q", got.Hostname, "dev-machine")
	}
	if got.GitRemote != "git@github.com:org/repo.git" {
		t.Errorf("GitRemote = %q, want %q", got.GitRemote, "git@github.com:org/repo.git")
	}
	if got.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if got.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestCreateWorkspacePath_Duplicate(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

	wp1 := &types.WorkspacePath{
		WorkspaceID: ws.ID,
		Path:        "/home/user/projects/myapp",
	}
	if err := store.CreateWorkspacePath(ctx, wp1); err != nil {
		t.Fatalf("first CreateWorkspacePath failed: %v", err)
	}

	wp2 := &types.WorkspacePath{
		WorkspaceID: ws.ID,
		Path:        "/home/user/projects/myapp",
	}
	err := store.CreateWorkspacePath(ctx, wp2)
	if err == nil {
		t.Fatal("expected error creating duplicate workspace path, got nil")
	}
}

func TestListWorkspacePaths(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

	paths := []string{
		"/home/user/projects/app-a",
		"/home/user/projects/app-b",
		"/home/user/projects/app-c",
	}
	for _, p := range paths {
		wp := &types.WorkspacePath{
			WorkspaceID: ws.ID,
			Path:        p,
		}
		if err := store.CreateWorkspacePath(ctx, wp); err != nil {
			t.Fatalf("CreateWorkspacePath(%s) failed: %v", p, err)
		}
	}

	// Create another workspace with a path to ensure filtering works
	ws2 := &types.Workspace{Name: "Other Workspace", Prefix: "oth"}
	if err := store.CreateWorkspace(ctx, ws2); err != nil {
		t.Fatalf("create other workspace: %v", err)
	}
	otherWP := &types.WorkspacePath{
		WorkspaceID: ws2.ID,
		Path:        "/home/user/projects/other",
	}
	if err := store.CreateWorkspacePath(ctx, otherWP); err != nil {
		t.Fatalf("CreateWorkspacePath(other) failed: %v", err)
	}

	got, err := store.ListWorkspacePaths(ctx, ws.ID)
	if err != nil {
		t.Fatalf("ListWorkspacePaths failed: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("ListWorkspacePaths returned %d paths, want 3", len(got))
	}

	// Should be ordered by path
	for i, wp := range got {
		if wp.Path != paths[i] {
			t.Errorf("paths[%d] = %q, want %q", i, wp.Path, paths[i])
		}
	}
}

func TestUpdateWorkspacePath(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

	wp := &types.WorkspacePath{
		WorkspaceID: ws.ID,
		Path:        "/home/user/projects/myapp",
		Label:       "original",
	}
	if err := store.CreateWorkspacePath(ctx, wp); err != nil {
		t.Fatalf("CreateWorkspacePath failed: %v", err)
	}

	// Update fields
	wp.Label = "updated-label"
	wp.Hostname = "new-host"
	wp.GitRemote = "git@github.com:new/repo.git"

	if err := store.UpdateWorkspacePath(ctx, wp); err != nil {
		t.Fatalf("UpdateWorkspacePath failed: %v", err)
	}

	got, err := store.GetWorkspacePath(ctx, wp.ID)
	if err != nil {
		t.Fatalf("GetWorkspacePath failed: %v", err)
	}

	if got.Label != "updated-label" {
		t.Errorf("Label = %q, want %q", got.Label, "updated-label")
	}
	if got.Hostname != "new-host" {
		t.Errorf("Hostname = %q, want %q", got.Hostname, "new-host")
	}
	if got.GitRemote != "git@github.com:new/repo.git" {
		t.Errorf("GitRemote = %q, want %q", got.GitRemote, "git@github.com:new/repo.git")
	}
}

func TestDeleteWorkspacePath(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

	wp := &types.WorkspacePath{
		WorkspaceID: ws.ID,
		Path:        "/home/user/projects/myapp",
	}
	if err := store.CreateWorkspacePath(ctx, wp); err != nil {
		t.Fatalf("CreateWorkspacePath failed: %v", err)
	}

	if err := store.DeleteWorkspacePath(ctx, wp.ID); err != nil {
		t.Fatalf("DeleteWorkspacePath failed: %v", err)
	}

	_, err := store.GetWorkspacePath(ctx, wp.ID)
	if err == nil {
		t.Fatal("expected error getting deleted workspace path, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestResolveWorkspaceByPath(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

	wp := &types.WorkspacePath{
		WorkspaceID: ws.ID,
		Path:        "/home/user/projects/myapp",
		Label:       "main-dir",
	}
	if err := store.CreateWorkspacePath(ctx, wp); err != nil {
		t.Fatalf("CreateWorkspacePath failed: %v", err)
	}

	got, err := store.ResolveWorkspaceByPath(ctx, "/home/user/projects/myapp")
	if err != nil {
		t.Fatalf("ResolveWorkspaceByPath failed: %v", err)
	}

	if got.ID != wp.ID {
		t.Errorf("ID = %q, want %q", got.ID, wp.ID)
	}
	if got.WorkspaceID != ws.ID {
		t.Errorf("WorkspaceID = %q, want %q", got.WorkspaceID, ws.ID)
	}
	if got.Label != "main-dir" {
		t.Errorf("Label = %q, want %q", got.Label, "main-dir")
	}
}

func TestResolveWorkspaceByPath_NotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	_, err := store.ResolveWorkspaceByPath(ctx, "/nonexistent/path")
	if err == nil {
		t.Fatal("expected error resolving non-existent path, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestUpdatePathLastAccessed(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	ws := setupTestWorkspace(t, store)

	wp := &types.WorkspacePath{
		WorkspaceID: ws.ID,
		Path:        "/home/user/projects/myapp",
	}
	if err := store.CreateWorkspacePath(ctx, wp); err != nil {
		t.Fatalf("CreateWorkspacePath failed: %v", err)
	}

	// Initially last_accessed_at should be nil
	got, err := store.GetWorkspacePath(ctx, wp.ID)
	if err != nil {
		t.Fatalf("GetWorkspacePath failed: %v", err)
	}
	if got.LastAccessedAt != nil {
		t.Errorf("expected LastAccessedAt to be nil initially, got %v", got.LastAccessedAt)
	}

	before := time.Now().Add(-time.Second)

	if err := store.UpdatePathLastAccessed(ctx, wp.ID); err != nil {
		t.Fatalf("UpdatePathLastAccessed failed: %v", err)
	}

	after := time.Now().Add(time.Second)

	got, err = store.GetWorkspacePath(ctx, wp.ID)
	if err != nil {
		t.Fatalf("GetWorkspacePath after update failed: %v", err)
	}

	if got.LastAccessedAt == nil {
		t.Fatal("expected LastAccessedAt to be set after update")
	}
	if got.LastAccessedAt.Before(before) || got.LastAccessedAt.After(after) {
		t.Errorf("LastAccessedAt = %v, expected between %v and %v", got.LastAccessedAt, before, after)
	}
}
