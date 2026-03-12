//go:build integration

package integration

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// arcCmdInDir runs the arc binary with the given arguments, setting both
// the HOME directory (for config isolation) and the working directory.
func arcCmdInDir(t *testing.T, homeDir, workDir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(arcBinary, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"ARC_SERVER="+serverURL,
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// arcCmdInDirSuccess runs the arc binary in the given working directory
// and fails the test if the command returns a non-zero exit code.
func arcCmdInDirSuccess(t *testing.T, homeDir, workDir string, args ...string) string {
	t.Helper()
	output, err := arcCmdInDir(t, homeDir, workDir, args...)
	if err != nil {
		t.Fatalf("arc %v failed: %v\noutput: %s", args, err, output)
	}
	return output
}

// TestInitNameBasedLookup verifies that initializing with the same project
// name from two different directories reuses the same project. Issues
// created from one directory should be visible from the other.
func TestInitNameBasedLookup(t *testing.T) {
	home := setupHome(t)

	dirA := t.TempDir()
	dirB := t.TempDir()

	// Init project "shared-proj" from directory A.
	outA := arcCmdInDirSuccess(t, home, dirA, "init", "shared-proj", "--server", serverURL)
	if !strings.Contains(strings.ToLower(outA), "project") {
		t.Errorf("expected project mention in init output from dir A, got: %s", outA)
	}

	// Init project "shared-proj" from directory B — should reuse the same project.
	outB := arcCmdInDirSuccess(t, home, dirB, "init", "shared-proj", "--server", serverURL)
	if !strings.Contains(strings.ToLower(outB), "project") {
		t.Errorf("expected project mention in init output from dir B, got: %s", outB)
	}

	// Create an issue from directory A.
	createOut := arcCmdInDirSuccess(t, home, dirA, "create", "Shared issue from A", "--type", "task", "--server", serverURL)
	id, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	// List issues from directory B — should see the issue created from A.
	listOut := arcCmdInDirSuccess(t, home, dirB, "list", "--server", serverURL)
	if !strings.Contains(listOut, "Shared issue from A") {
		t.Errorf("expected issue created from dir A to appear in dir B list, got: %s", listOut)
	}

	// Show the issue from directory B by ID.
	showOut := arcCmdInDirSuccess(t, home, dirB, "show", id, "--server", serverURL)
	if !strings.Contains(showOut, "Shared issue from A") {
		t.Errorf("expected issue details from dir B, got: %s", showOut)
	}
}

// TestInitAutoGenerateName verifies that running `arc init` without a name
// auto-generates a project name, and running it again from the same
// directory reuses the same project.
func TestInitAutoGenerateName(t *testing.T) {
	home := setupHome(t)

	dir := t.TempDir()

	// Init without a name — should auto-generate one.
	out1 := arcCmdInDirSuccess(t, home, dir, "init", "--server", serverURL)
	if !strings.Contains(strings.ToLower(out1), "project") {
		t.Errorf("expected project mention in init output, got: %s", out1)
	}

	// Create an issue so we can verify project identity.
	createOut := arcCmdInDirSuccess(t, home, dir, "create", "Auto-gen project issue", "--type", "task", "--server", serverURL)
	_, ok := extractID(createOut)
	if !ok {
		t.Fatalf("could not extract issue ID from create output: %s", createOut)
	}

	// Init again from the same directory — should reuse the same project.
	out2 := arcCmdInDirSuccess(t, home, dir, "init", "--server", serverURL)
	if !strings.Contains(strings.ToLower(out2), "project") {
		t.Errorf("expected project mention in second init output, got: %s", out2)
	}

	// List should still show the previously created issue (same project).
	listOut := arcCmdInDirSuccess(t, home, dir, "list", "--server", serverURL)
	if !strings.Contains(listOut, "Auto-gen project issue") {
		t.Errorf("expected issue to persist after re-init, got: %s", listOut)
	}
}
