package planfiles

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

const (
	lockName      = ".maintenance.lock"
	statusSymlink = "symlink"
)

// ErrBusy means publishers or another maintenance operation hold the root.
var ErrBusy = errors.New("plan root busy: stop writers before offline maintenance")

// lockRoot uses a kernel lock on a stable inode. Never unlink the lock file:
// independent processes must coordinate on the same inode through DB commit.
func lockRoot(root *os.Root, exclusive, create bool) (*os.File, error) {
	// Never follow a replaced lock symlink; all participants use this stable inode.
	flags := os.O_RDWR | syscall.O_NOFOLLOW
	if create {
		flags |= os.O_CREATE
	}
	f, err := root.OpenFile(lockName, flags, contentPerm)
	if err != nil {
		return nil, fmt.Errorf("open maintenance lock (initialize root with the upgraded server first): %w", err)
	}
	// A shared lifetime lock permits normal concurrent publishers.
	mode := syscall.LOCK_SH
	if exclusive {
		mode = syscall.LOCK_EX
	}
	if err := syscall.Flock(int(f.Fd()), mode|syscall.LOCK_NB); err != nil {
		return nil, errors.Join(ErrBusy, err, f.Close())
	}
	return f, nil
}

// Maintenance owns exclusive access until Close, including the interval between
// inventory reads, file operations, and a SQLite backup. Opening never creates
// directories, lock files or probes, so dry-run is non-mutating.
type Maintenance struct {
	store *Store
	lock  *os.File
	path  string
}

// OpenMaintenance requires an initialized non-symlink root and acquires a
// nonblocking exclusive lock before exposing any maintenance operation.
func OpenMaintenance(path string) (*Maintenance, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("plan root must be absolute")
	}
	info, err := os.Lstat(path) //nolint:gosec // Explicit absolute operator root; checked before capability open.
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("plan root must be a non-symlink directory")
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	// The root capability confines subsequent walking, reads and deletes.
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	lock, err := lockRoot(root, true, false)
	if err != nil {
		return nil, errors.Join(err, root.Close())
	}
	return &Maintenance{store: &Store{root: root, fs: rootFS{root}}, lock: lock, path: canonical}, nil
}

// Close releases the filesystem capability before allowing publishers to start.
func (m *Maintenance) Close() error { return errors.Join(m.store.root.Close(), m.lock.Close()) }

// Inspection describes content without exposing it or following symlinks.
type Inspection struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func (m *Maintenance) Inspect(ctx context.Context, references []Blob) ([]Inspection, error) {
	// Start with committed references so missing files cannot disappear from the report.
	reports := make([]Inspection, 0)
	referenced := map[string]bool{}
	for _, blob := range references {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		referenced[blob.RelativePath] = true
		report := m.inspectReference(ctx, blob)
		reports = append(reports, report)
	}
	// WalkDir does not follow symlinks, including directory symlinks.
	err := fs.WalkDir(m.store.root.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == "." || path == lockName {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			reports = append(reports, Inspection{Path: path, Status: statusSymlink, Error: "symlinks are not followed"})
		} else if !entry.IsDir() && !referenced[path] {
			status := "orphan"
			if !entry.Type().IsRegular() {
				status = "unsupported"
			}
			reports = append(reports, Inspection{Path: path, Status: status})
		}
		return nil
	})
	sort.SliceStable(reports, func(i, j int) bool { return reports[i].Path < reports[j].Path })
	return reports, err
}

// inspectReference uses the same verified reader as API approval and reads.
// Missing content is distinct from a digest, size or confinement mismatch.
func (m *Maintenance) inspectReference(ctx context.Context, blob Blob) Inspection {
	report := Inspection{Path: blob.RelativePath, Status: "ok"}
	if _, err := m.store.Read(ctx, blob); err != nil {
		report.Status = "mismatched"
		if errors.Is(err, os.ErrNotExist) {
			report.Status = "missing"
		}
		report.Error = err.Error()
	}
	return report
}

// Cleanup rechecks the selected report against current references while still
// holding exclusion. Unselected or newly referenced files are never removed.
func (m *Maintenance) Cleanup(ctx context.Context, references []Blob, selected []string, dryRun bool) error {
	if len(selected) == 0 {
		return errors.New("cleanup requires an explicit orphan selection from a dry-run report")
	}
	reports, err := m.Inspect(ctx, references)
	if err != nil {
		return err
	}
	// Only current regular orphan paths are eligible, never arbitrary report paths.
	orphans := map[string]bool{}
	for _, r := range reports {
		if r.Status == "orphan" {
			orphans[r.Path] = true
		}
	}
	// Validate the entire selection before deleting any file.
	for _, path := range selected {
		if !orphans[path] {
			return fmt.Errorf("selected path %q is not a current regular orphan", path)
		}
	}
	if dryRun {
		return nil
	}
	// Keep exclusion held through each unlink and parent directory flush.
	for _, path := range selected {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := m.store.root.Remove(path); err != nil {
			return err
		}
		if err := m.store.fs.SyncDir(filepath.Dir(path)); err != nil {
			return err
		}
	}
	return nil
}

// CopyTo copies the full content tree, including orphans, into a new staging
// root while exclusion is held. Symlinks and special files fail the backup.
func (m *Maintenance) CopyTo(ctx context.Context, destination string) error {
	if err := m.CheckDestination(destination); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Mkdir(destination, directoryPerm); err != nil {
		return err
	}
	// Preserve every regular artifact, including uncertain publication orphans.
	return fs.WalkDir(m.store.root.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		target := filepath.Join(destination, path)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup rejects symlink %q", path)
		}
		if entry.IsDir() {
			return os.Mkdir(target, directoryPerm)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("backup rejects special file %q", path)
		}
		source, err := m.store.root.Open(path)
		if err != nil {
			return err
		}
		dest, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, contentPerm)
		if err != nil {
			return errors.Join(err, source.Close())
		}
		_, copyErr := io.Copy(dest, source)
		return errors.Join(copyErr, dest.Sync(), dest.Close(), source.Close())
	})
}

// CheckDestination rejects backups within the source tree, including aliases,
// before any output is created. Its parent must already exist.
func (m *Maintenance) CheckDestination(destination string) error {
	if !filepath.IsAbs(destination) {
		return errors.New("backup destination must be absolute")
	}
	// Resolve aliases in the output parent before checking source containment.
	parent, err := filepath.EvalSymlinks(filepath.Dir(destination))
	if err != nil {
		return err
	}
	target := filepath.Join(parent, filepath.Base(destination))
	rel, err := filepath.Rel(m.path, target)
	if err != nil {
		return err
	}
	if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return errors.New("backup destination must be outside the plan root")
	}
	return nil
}
