package sqlite_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sentiolabs/arc/internal/storage/sqlite"
	"github.com/sentiolabs/arc/internal/types"
)

const testPathTypeSymlink = "symlink"

func TestCreateWorkspace(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	ws := &types.Workspace{
		ProjectID: proj.ID,
		Path:      "/home/user/projects/myapp",
		Label:     "main",
		Hostname:  "dev-machine",
		GitRemote: "git@github.com:org/repo.git",
	}

	err := store.CreateWorkspace(ctx, ws)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	// ID should be generated
	if ws.ID == "" {
		t.Error("expected ID to be generated")
	}

	// Retrieve and verify
	got, err := store.GetWorkspace(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetWorkspace failed: %v", err)
	}

	if got.ProjectID != proj.ID {
		t.Errorf("ProjectID = %q, want %q", got.ProjectID, proj.ID)
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

func TestCreateWorkspace_Duplicate(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	ws1 := &types.Workspace{
		ProjectID: proj.ID,
		Path:      "/home/user/projects/myapp",
	}
	if err := store.CreateWorkspace(ctx, ws1); err != nil {
		t.Fatalf("first CreateWorkspace failed: %v", err)
	}

	ws2 := &types.Workspace{
		ProjectID: proj.ID,
		Path:      "/home/user/projects/myapp",
	}
	err := store.CreateWorkspace(ctx, ws2)
	if err == nil {
		t.Fatal("expected error creating duplicate workspace, got nil")
	}
}

func TestListWorkspaces(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	paths := []string{
		"/home/user/projects/app-a",
		"/home/user/projects/app-b",
		"/home/user/projects/app-c",
	}
	for _, p := range paths {
		ws := &types.Workspace{
			ProjectID: proj.ID,
			Path:      p,
		}
		if err := store.CreateWorkspace(ctx, ws); err != nil {
			t.Fatalf("CreateWorkspace(%s) failed: %v", p, err)
		}
	}

	// Create another project with a workspace to ensure filtering works
	proj2 := &types.Project{Name: "Other Project", Prefix: "oth"}
	if err := store.CreateProject(ctx, proj2); err != nil {
		t.Fatalf("create other project: %v", err)
	}
	otherWS := &types.Workspace{
		ProjectID: proj2.ID,
		Path:      "/home/user/projects/other",
	}
	if err := store.CreateWorkspace(ctx, otherWS); err != nil {
		t.Fatalf("CreateWorkspace(other) failed: %v", err)
	}

	got, err := store.ListWorkspaces(ctx, proj.ID)
	if err != nil {
		t.Fatalf("ListWorkspaces failed: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("ListWorkspaces returned %d workspaces, want 3", len(got))
	}

	// Should be ordered by path
	for i, ws := range got {
		if ws.Path != paths[i] {
			t.Errorf("paths[%d] = %q, want %q", i, ws.Path, paths[i])
		}
	}
}

func TestUpdateWorkspace(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	ws := &types.Workspace{
		ProjectID: proj.ID,
		Path:      "/home/user/projects/myapp",
		Label:     "original",
	}
	if err := store.CreateWorkspace(ctx, ws); err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	// Update fields
	ws.Label = "updated-label"
	ws.Hostname = "new-host"
	ws.GitRemote = "git@github.com:new/repo.git"

	if err := store.UpdateWorkspace(ctx, ws); err != nil {
		t.Fatalf("UpdateWorkspace failed: %v", err)
	}

	got, err := store.GetWorkspace(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetWorkspace failed: %v", err)
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

func TestDeleteWorkspace(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	ws := &types.Workspace{
		ProjectID: proj.ID,
		Path:      "/home/user/projects/myapp",
	}
	if err := store.CreateWorkspace(ctx, ws); err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	if err := store.DeleteWorkspace(ctx, ws.ID); err != nil {
		t.Fatalf("DeleteWorkspace failed: %v", err)
	}

	_, err := store.GetWorkspace(ctx, ws.ID)
	if err == nil {
		t.Fatal("expected error getting deleted workspace, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestResolveProjectByPath(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	ws := &types.Workspace{
		ProjectID: proj.ID,
		Path:      "/home/user/projects/myapp",
		Label:     "main-dir",
	}
	if err := store.CreateWorkspace(ctx, ws); err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	got, err := store.ResolveProjectByPath(ctx, "/home/user/projects/myapp")
	if err != nil {
		t.Fatalf("ResolveProjectByPath failed: %v", err)
	}

	if got.ID != ws.ID {
		t.Errorf("ID = %q, want %q", got.ID, ws.ID)
	}
	if got.ProjectID != proj.ID {
		t.Errorf("ProjectID = %q, want %q", got.ProjectID, proj.ID)
	}
	if got.Label != "main-dir" {
		t.Errorf("Label = %q, want %q", got.Label, "main-dir")
	}
}

func TestResolveProjectByPath_NotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	_, err := store.ResolveProjectByPath(ctx, "/nonexistent/path")
	if err == nil {
		t.Fatal("expected error resolving non-existent path, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestResolveProjectByPath_DualPathVariants(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	// Register two paths for the same project (simulating symlink + resolved)
	symlinkWS := &types.Workspace{
		ProjectID: proj.ID,
		Path:      "/Users/dev/devspace/project",
		Label:     "project",
	}
	resolvedWS := &types.Workspace{
		ProjectID: proj.ID,
		Path:      "/Volumes/ExternalSSD/devspace/project",
		Label:     "project (resolved)",
	}

	if err := store.CreateWorkspace(ctx, symlinkWS); err != nil {
		t.Fatalf("CreateWorkspace(symlink) failed: %v", err)
	}
	if err := store.CreateWorkspace(ctx, resolvedWS); err != nil {
		t.Fatalf("CreateWorkspace(resolved) failed: %v", err)
	}

	// Both paths should resolve to the same project
	got1, err := store.ResolveProjectByPath(ctx, "/Users/dev/devspace/project")
	if err != nil {
		t.Fatalf("ResolveProjectByPath(symlink) failed: %v", err)
	}
	if got1.ProjectID != proj.ID {
		t.Errorf("symlink path resolved to project %q, want %q", got1.ProjectID, proj.ID)
	}

	got2, err := store.ResolveProjectByPath(ctx, "/Volumes/ExternalSSD/devspace/project")
	if err != nil {
		t.Fatalf("ResolveProjectByPath(resolved) failed: %v", err)
	}
	if got2.ProjectID != proj.ID {
		t.Errorf("resolved path resolved to project %q, want %q", got2.ProjectID, proj.ID)
	}
}

func TestCreateWorkspace_WithPathType(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	// Create a canonical path
	canonical := &types.Workspace{
		ProjectID: proj.ID,
		Path:      "/Volumes/ExternalSSD/devspace/project",
		Label:     "project",
		PathType:  "canonical",
	}
	if err := store.CreateWorkspace(ctx, canonical); err != nil {
		t.Fatalf("CreateWorkspace(canonical) failed: %v", err)
	}

	got, err := store.GetWorkspace(ctx, canonical.ID)
	if err != nil {
		t.Fatalf("GetWorkspace failed: %v", err)
	}
	if got.PathType != "canonical" {
		t.Errorf("PathType = %q, want %q", got.PathType, "canonical")
	}

	// Create a symlink path
	symlink := &types.Workspace{
		ProjectID: proj.ID,
		Path:      "/Users/dev/devspace/project",
		Label:     "project",
		PathType:  testPathTypeSymlink,
	}
	if err := store.CreateWorkspace(ctx, symlink); err != nil {
		t.Fatalf("CreateWorkspace(symlink) failed: %v", err)
	}

	got2, err := store.GetWorkspace(ctx, symlink.ID)
	if err != nil {
		t.Fatalf("GetWorkspace failed: %v", err)
	}
	if got2.PathType != testPathTypeSymlink {
		t.Errorf("PathType = %q, want %q", got2.PathType, testPathTypeSymlink)
	}
}

func TestCreateWorkspace_DefaultPathType(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	// Create a workspace without setting PathType — should default to "canonical"
	ws := &types.Workspace{
		ProjectID: proj.ID,
		Path:      "/home/user/projects/myapp",
	}
	if err := store.CreateWorkspace(ctx, ws); err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	got, err := store.GetWorkspace(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetWorkspace failed: %v", err)
	}
	if got.PathType != "canonical" {
		t.Errorf("PathType = %q, want %q", got.PathType, "canonical")
	}
}

func TestUpdateWorkspace_PathType(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	ws := &types.Workspace{
		ProjectID: proj.ID,
		Path:      "/home/user/projects/myapp",
		PathType:  "canonical",
	}
	if err := store.CreateWorkspace(ctx, ws); err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	// Update path_type to symlink
	ws.PathType = testPathTypeSymlink
	if err := store.UpdateWorkspace(ctx, ws); err != nil {
		t.Fatalf("UpdateWorkspace failed: %v", err)
	}

	got, err := store.GetWorkspace(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetWorkspace failed: %v", err)
	}
	if got.PathType != testPathTypeSymlink {
		t.Errorf("PathType = %q, want %q", got.PathType, testPathTypeSymlink)
	}
}

func TestUpdateWorkspaceLastAccessed(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	ws := &types.Workspace{
		ProjectID: proj.ID,
		Path:      "/home/user/projects/myapp",
	}
	if err := store.CreateWorkspace(ctx, ws); err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	// Initially last_accessed_at should be nil
	got, err := store.GetWorkspace(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetWorkspace failed: %v", err)
	}
	if got.LastAccessedAt != nil {
		t.Errorf("expected LastAccessedAt to be nil initially, got %v", got.LastAccessedAt)
	}

	before := time.Now().Add(-time.Second)

	if err := store.UpdateWorkspaceLastAccessed(ctx, ws.ID); err != nil {
		t.Fatalf("UpdateWorkspaceLastAccessed failed: %v", err)
	}

	after := time.Now().Add(time.Second)

	got, err = store.GetWorkspace(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetWorkspace after update failed: %v", err)
	}

	if got.LastAccessedAt == nil {
		t.Fatal("expected LastAccessedAt to be set after update")
	}
	if got.LastAccessedAt.Before(before) || got.LastAccessedAt.After(after) {
		t.Errorf("LastAccessedAt = %v, expected between %v and %v", got.LastAccessedAt, before, after)
	}
}

func TestResolveProjectByPath_PrefixMatch(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	ws := &types.Workspace{
		ProjectID: proj.ID,
		Path:      "/home/user/projects/myapp",
		Label:     "main",
	}
	if err := store.CreateWorkspace(ctx, ws); err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	got, err := store.ResolveProjectByPath(ctx, "/home/user/projects/myapp/.worktrees/feature-x/internal/api")
	if err != nil {
		t.Fatalf("ResolveProjectByPath(subdir) failed: %v", err)
	}
	if got.ID != ws.ID {
		t.Errorf("ID = %q, want %q (longest-prefix match)", got.ID, ws.ID)
	}
}

func TestResolveProjectByPath_LongestPrefixWins(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	projOuter := setupTestProject(t, store)
	projInner := setupTestProjectNamed(t, store, "inner-project", "inner")

	wsOuter := &types.Workspace{ProjectID: projOuter.ID, Path: "/repos/outer", Label: "outer"}
	wsInner := &types.Workspace{ProjectID: projInner.ID, Path: "/repos/outer/sub/inner", Label: "inner"}
	if err := store.CreateWorkspace(ctx, wsOuter); err != nil {
		t.Fatalf("CreateWorkspace(outer) failed: %v", err)
	}
	if err := store.CreateWorkspace(ctx, wsInner); err != nil {
		t.Fatalf("CreateWorkspace(inner) failed: %v", err)
	}

	got, err := store.ResolveProjectByPath(ctx, "/repos/outer/sub/inner/deep/file")
	if err != nil {
		t.Fatalf("ResolveProjectByPath(nested) failed: %v", err)
	}
	if got.ID != wsInner.ID {
		t.Errorf("ID = %q, want %q (longest prefix should win)", got.ID, wsInner.ID)
	}
}

func TestResolveProjectByPath_ComponentBoundary(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	ws := &types.Workspace{ProjectID: proj.ID, Path: "/repos/proj", Label: "proj"}
	if err := store.CreateWorkspace(ctx, ws); err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	// "/repos/proj-foo" must NOT match "/repos/proj" — different directories.
	_, err := store.ResolveProjectByPath(ctx, "/repos/proj-foo/file")
	if err == nil {
		t.Fatal("ResolveProjectByPath(/repos/proj-foo/file) should not match /repos/proj")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestResolveProjectByPath_LiteralUnderscoreInRegisteredPath(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	ws := &types.Workspace{
		ProjectID: proj.ID,
		Path:      "/home/john_doe/proj",
		Label:     "with-underscore",
	}
	if err := store.CreateWorkspace(ctx, ws); err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	// Exact match must work.
	got, err := store.ResolveProjectByPath(ctx, "/home/john_doe/proj")
	if err != nil {
		t.Fatalf("exact match failed: %v", err)
	}
	if got.ID != ws.ID {
		t.Errorf("exact match resolved to %q, want %q", got.ID, ws.ID)
	}

	// Subdirectory of the real path must work.
	got, err = store.ResolveProjectByPath(ctx, "/home/john_doe/proj/sub/file")
	if err != nil {
		t.Fatalf("subdir of real path failed: %v", err)
	}
	if got.ID != ws.ID {
		t.Errorf("subdir of real path resolved to %q, want %q", got.ID, ws.ID)
	}

	// A path where the underscore position has a different character must NOT match.
	// Without REPLACE-escaping, '_' acts as a single-char wildcard and "/home/johnXdoe/projXfile"
	// would falsely match "/home/john_doe/proj".
	if _, err := store.ResolveProjectByPath(ctx, "/home/johnXdoe/proj/file"); err == nil {
		t.Fatal("path with X where _ was registered must NOT match (LIKE wildcard leak)")
	}
}

func TestResolveProjectByPath_LiteralPercentInRegisteredPath(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	ws := &types.Workspace{
		ProjectID: proj.ID,
		Path:      "/repos/100%coverage",
		Label:     "with-percent",
	}
	if err := store.CreateWorkspace(ctx, ws); err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	// Subdirectory of the real path must work.
	got, err := store.ResolveProjectByPath(ctx, "/repos/100%coverage/sub")
	if err != nil {
		t.Fatalf("subdir of percent path failed: %v", err)
	}
	if got.ID != ws.ID {
		t.Errorf("subdir resolved to %q, want %q", got.ID, ws.ID)
	}

	// Without escaping, % would match anything: "/repos/100ANYcoverage/file" should NOT match.
	if _, err := store.ResolveProjectByPath(ctx, "/repos/100ANYTHINGcoverage/file"); err == nil {
		t.Fatal("path that differs only by what % covers must NOT match")
	}
}

func TestResolveProjectByPath_LiteralBackslashInRegisteredPath(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	proj := setupTestProject(t, store)

	// Edge case: path containing a literal backslash. Rare on Linux/Mac but possible.
	ws := &types.Workspace{
		ProjectID: proj.ID,
		Path:      `/repos/weird\name/proj`,
		Label:     "with-backslash",
	}
	if err := store.CreateWorkspace(ctx, ws); err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	got, err := store.ResolveProjectByPath(ctx, `/repos/weird\name/proj/sub`)
	if err != nil {
		t.Fatalf("subdir of backslash path failed: %v", err)
	}
	if got.ID != ws.ID {
		t.Errorf("subdir resolved to %q, want %q", got.ID, ws.ID)
	}
}

func setupTestProjectNamed(t *testing.T, store *sqlite.Store, name, prefix string) *types.Project {
	t.Helper()
	proj := &types.Project{
		Name:   name,
		Prefix: prefix,
	}
	if err := store.CreateProject(context.Background(), proj); err != nil {
		t.Fatalf("CreateProject(%s) failed: %v", name, err)
	}
	return proj
}
