package vcs_test

import (
	"path/filepath"
	"testing"

	"github.com/sentiolabs/arc/internal/testutil/jjtest"
	"github.com/sentiolabs/arc/internal/vcs"
)

func TestDetectRemote_NativeJJ(t *testing.T) {
	jjtest.RequireJJ(t)
	dir := jjtest.InitNative(t, filepath.Join(t.TempDir(), "repo"))
	jjtest.AddRemote(t, dir, "origin", "git@example.com:org/native.git")

	if got := vcs.DetectRemote(dir); got != "git@example.com:org/native.git" {
		t.Fatalf("DetectRemote(native jj) = %q, want the origin URL", got)
	}
}

func TestDetectMainRepo_NativeJJSecondaryWorkspace(t *testing.T) {
	jjtest.RequireJJ(t)
	root := t.TempDir()
	main := jjtest.InitNative(t, filepath.Join(root, "main"))
	ws := filepath.Join(root, "ws")
	jjtest.AddWorkspace(t, main, ws, "ws")

	got := vcs.DetectMainRepo(ws)
	// vcs canonicalizes via core.NormalizePath; compare against the resolved main.
	want := mustEvalSymlinks(t, main)
	if got != want {
		t.Fatalf("DetectMainRepo(secondary jj workspace) = %q, want %q", got, want)
	}
}

func TestDetectRemote_NativeJJFromSecondaryWorkspace(t *testing.T) {
	jjtest.RequireJJ(t)
	root := t.TempDir()
	main := jjtest.InitNative(t, filepath.Join(root, "main"))
	jjtest.AddRemote(t, main, "origin", "git@example.com:org/shared.git")
	ws := filepath.Join(root, "ws")
	jjtest.AddWorkspace(t, main, ws, "ws")

	// From the secondary workspace, DetectRemote follows .jj/repo to the shared
	// repo and reads its git_target backend.
	if got := vcs.DetectRemote(ws); got != "git@example.com:org/shared.git" {
		t.Fatalf("DetectRemote(secondary workspace) = %q, want shared origin", got)
	}
}

func mustEvalSymlinks(t *testing.T, p string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", p, err)
	}
	return resolved
}
