package planfiles

import (
	"errors"
	"os"
	"path/filepath"
)

// PathWithinRoot checks physical filesystem ancestry, including the root itself.
// Symlink resolution alone does not normalize spelling on case-insensitive
// filesystems, so every existing ancestor is compared by identity with SameFile.
// For a new destination or sidecar, its nearest existing ancestor establishes
// containment. Dangling links and inaccessible paths fail closed.
func PathWithinRoot(root, path string) (bool, error) {
	if !filepath.IsAbs(root) || !filepath.IsAbs(path) {
		return false, errors.New("containment paths must be absolute")
	}
	rootInfo, err := os.Stat(root)
	if err != nil {
		return false, err
	}
	if !rootInfo.IsDir() {
		return false, errors.New("containment root must be a directory")
	}
	existing, err := existingAncestor(path)
	if err != nil {
		return false, err
	}
	// Resolve symlink targets before walking parents: lexical parents of a final
	// symlink would otherwise describe the alias location rather than its target.
	current, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return false, err
	}
	for {
		info, err := os.Stat(current)
		if err != nil {
			return false, err
		}
		if os.SameFile(rootInfo, info) {
			return true, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false, nil
		}
		current = parent
	}
}

// existingAncestor ascends only genuinely absent path components. An existing
// dangling symlink is not an absent leaf and cannot establish safe containment.
func existingAncestor(path string) (string, error) {
	for {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			if err != nil {
				return "", err
			}
			return "", errors.New("containment path contains a dangling symlink")
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", os.ErrNotExist
		}
		path = parent
	}
}
