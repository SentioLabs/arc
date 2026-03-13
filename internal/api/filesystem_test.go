package api //nolint:testpackage // tests use internal helpers that access unexported fields

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestBrowseFilesystem(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	// Create temp dir with subdirectories
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "projectA"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "projectB"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "projectC"), 0o755); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/browse?dir="+tmpDir, nil)
	rec := httptest.NewRecorder()
	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var entries []browseEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Should be sorted alphabetically
	if entries[0].Name != "projectA" {
		t.Errorf("entries[0].Name = %q, want %q", entries[0].Name, "projectA")
	}
	if entries[1].Name != "projectB" {
		t.Errorf("entries[1].Name = %q, want %q", entries[1].Name, "projectB")
	}
	if entries[2].Name != "projectC" {
		t.Errorf("entries[2].Name = %q, want %q", entries[2].Name, "projectC")
	}

	// Verify paths are absolute
	if !filepath.IsAbs(entries[0].Path) {
		t.Errorf("expected absolute path, got %q", entries[0].Path)
	}

	// All should be dirs
	for _, e := range entries {
		if !e.IsDir {
			t.Errorf("entry %q should be a directory", e.Name)
		}
	}
}

func TestBrowseFilesystem_DetectsGitRepo(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	tmpDir := t.TempDir()
	// Create a child dir that is a git repo
	gitProject := filepath.Join(tmpDir, "myrepo")
	if err := os.MkdirAll(filepath.Join(gitProject, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a child dir that is NOT a git repo
	if err := os.MkdirAll(filepath.Join(tmpDir, "notrepo"), 0o755); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/browse?dir="+tmpDir, nil)
	rec := httptest.NewRecorder()
	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var entries []browseEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Find myrepo entry
	var myrepo, notrepo *browseEntry
	for i := range entries {
		switch entries[i].Name {
		case "myrepo":
			myrepo = &entries[i]
		case "notrepo":
			notrepo = &entries[i]
		}
	}

	if myrepo == nil {
		t.Fatal("myrepo entry not found")
	}
	if !myrepo.IsGitRepo {
		t.Error("myrepo should be detected as a git repo")
	}

	if notrepo == nil {
		t.Fatal("notrepo entry not found")
	}
	if notrepo.IsGitRepo {
		t.Error("notrepo should not be detected as a git repo")
	}
}

func TestBrowseFilesystem_InvalidDir(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/filesystem/browse?dir=/nonexistent/path/that/does/not/exist", nil)
	rec := httptest.NewRecorder()
	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBrowseFilesystem_OnlyDirectories(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	tmpDir := t.TempDir()
	// Create mix of files and directories
	if err := os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "another.go"), []byte("package main"), 0o600); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/browse?dir="+tmpDir, nil)
	rec := httptest.NewRecorder()
	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var entries []browseEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (only dirs), got %d", len(entries))
	}

	if entries[0].Name != "subdir" {
		t.Errorf("expected 'subdir', got %q", entries[0].Name)
	}
}

func TestBrowseFilesystem_EmptyDir(t *testing.T) {
	server, cleanup := testServer(t)
	defer cleanup()

	tmpDir := t.TempDir()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/browse?dir="+tmpDir, nil)
	rec := httptest.NewRecorder()
	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var entries []browseEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}
