package planmigration

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var errDatabaseRootOverlap = errors.New("database and SQLite sidecars must be outside the plan root")

// checkDatabaseRootSeparation rejects roots that could classify live metadata as
// disposable blobs. Check both the configured spelling and its resolved target:
// SQLite sidecars may be beside either path when the database uses an alias.
// This runs before opening SQLite, producing candidates, copying or deleting.
func checkDatabaseRootSeparation(database, root string) error {
	if !filepath.IsAbs(database) || !filepath.IsAbs(root) {
		return errors.New("database and plan root paths must be absolute")
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	canonicalDB, err := filepath.EvalSymlinks(database)
	if err != nil {
		return err
	}
	for _, base := range []string{database, canonicalDB} {
		for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
			if err := checkProtectedPath(canonicalRoot, base+suffix); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkProtectedPath checks the resolved parent even for absent sidecars. For
// existing aliases it checks the target too. A dangling or inaccessible alias
// is an error, never evidence that metadata is safely outside the content root.
func checkProtectedPath(root, path string) error {
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return err
	}
	if err := rejectPathInRoot(root, filepath.Join(parent, filepath.Base(path))); err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if errors.Is(err, os.ErrNotExist) {
		if _, statErr := os.Lstat(path); errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
	}
	if err != nil {
		return err
	}
	return rejectPathInRoot(root, resolved)
}

// rejectPathInRoot compares complete path components, not string prefixes.
// A root itself cannot be protected metadata either. Sibling names are safe.
func rejectPathInRoot(root, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
		return fmt.Errorf("%w: %s", errDatabaseRootOverlap, path)
	}
	return nil
}
