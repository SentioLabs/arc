// Package planfiles publishes immutable Markdown below a server-owned root.
package planfiles

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

// MaxContentBytes is the maximum size of an uploaded Markdown revision.
const (
	MaxContentBytes             = 10 * 1024 * 1024
	directoryPerm   os.FileMode = 0o700
	contentPerm     os.FileMode = 0o600
)

// Blob is private storage metadata, never a client-selected filesystem path.
type Blob struct {
	RelativePath string
	SHA256       string
	Bytes        int64
}

// Publisher makes bytes durable before callers commit their database reference.
type Publisher interface {
	Publish(context.Context, string, string, int64, []byte) (Blob, error)
	Read(context.Context, Blob) ([]byte, error)
}

// IntegrityError indicates retained content is missing, inaccessible, or changed.
type IntegrityError struct {
	Path string
	Err  error
}

func (e *IntegrityError) Error() string {
	return fmt.Sprintf("plan content integrity failure for %q: %v", e.Path, e.Err)
}
func (e *IntegrityError) Unwrap() error { return e.Err }

// Store holds an open capability to the configured content directory.
// The operator owns this tree; published files must not be edited externally.
type Store struct {
	root  *os.Root
	fs    fileSystem
	newID func() string
}

var _ Publisher = (*Store)(nil)

// New opens an absolute content root, creating and flushing missing directories.
// It verifies write and sync access before the server starts accepting requests.
func New(path string) (*Store, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("plan root must be absolute: %q", path)
	}
	if err := createRoot(path); err != nil {
		return nil, fmt.Errorf("create plan root: %w", err)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, fmt.Errorf("open plan root: %w", err)
	}
	p := &Store{root: root, fs: rootFS{root}, newID: rand.Text}
	if err := p.checkWritable(); err != nil {
		return nil, errors.Join(fmt.Errorf("unusable plan root %q: %w", path, err), root.Close())
	}
	return p, nil
}

// Close releases the root capability after all requests have finished.
func (p *Store) Close() error { return p.root.Close() }

// createRoot flushes ancestry from the first existing directory downward.
// Opening the root afterward anchors all publication operations to this tree.
func createRoot(path string) error {
	// Record missing ancestors so each newly created entry's parent is flushed.
	var missing []string
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Stat(current)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("%q is not a directory", current)
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		missing = append(missing, current)
	}
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], directoryPerm); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		parent, err := os.Open(filepath.Dir(missing[i]))
		if err != nil {
			return err
		}
		if err := errors.Join(parent.Sync(), parent.Close()); err != nil {
			return err
		}
	}
	return nil
}

// checkWritable leaves no content artifact: it is only a startup access probe.
// Cleanup errors also fail startup instead of hiding an unusable directory.
func (p *Store) checkWritable() error {
	name := ".probe-" + p.newID()
	f, err := p.fs.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, contentPerm)
	if err != nil {
		return err
	}
	err = errors.Join(writeAll(f, []byte("probe")), f.Sync(), f.Close())
	return errors.Join(err, p.root.Remove(name), p.fs.SyncDir("."))
}

// Publish validates generated identity components, then writes and flushes bytes.
// Any failure returns a zero Blob. Unreferenced temporary/final files are retained
// for operator inspection; this layer never guesses a database commit outcome.
func (p *Store) Publish(ctx context.Context, projectID, planID string, revision int64, content []byte) (Blob, error) {
	if err := ctx.Err(); err != nil {
		return Blob{}, err
	}
	if err := validateUpload(projectID, planID, revision, content); err != nil {
		return Blob{}, err
	}
	publicationID := p.newID()
	if !validComponent(publicationID) {
		return Blob{}, errors.New("invalid publication identifier")
	}
	dir := filepath.Join(projectID, planID)
	for _, name := range []string{projectID, dir} {
		if err := p.ensureDir(name); err != nil {
			return Blob{}, err
		}
	}
	// Both names are server-generated siblings, keeping rename on one filesystem.
	filename := strconv.FormatInt(revision, 10) + "-" + publicationID
	relative := filepath.Join(dir, filename+".md")
	if _, err := p.fs.Lstat(relative); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			err = os.ErrExist
		}
		return Blob{}, fmt.Errorf("publication target: %w", err)
	}
	// Exclusive creation protects against a collision with an unfinished upload.
	temp := filepath.Join(dir, filename+".tmp")
	f, err := p.fs.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, contentPerm)
	if err != nil {
		return Blob{}, fmt.Errorf("create publication: %w", err)
	}
	if err := writeAll(f, content); err != nil {
		return Blob{}, errors.Join(fmt.Errorf("write publication: %w", err), f.Close())
	}
	if err := f.Sync(); err != nil {
		return Blob{}, errors.Join(fmt.Errorf("sync publication: %w", err), f.Close())
	}
	if err := f.Close(); err != nil {
		return Blob{}, fmt.Errorf("close publication: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Blob{}, err
	}
	// Only closed, flushed bytes become a final artifact; the directory flush
	// below is the final prerequisite for exposing metadata to the database.
	if err := p.fs.Rename(temp, relative); err != nil {
		return Blob{}, fmt.Errorf("rename publication: %w", err)
	}
	if err := p.fs.SyncDir(dir); err != nil {
		return Blob{}, fmt.Errorf("sync publication directory: %w", err)
	}
	sum := sha256.Sum256(content)
	return Blob{RelativePath: relative, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(content))}, nil
}

// ensureDir explicitly refuses symlinks even when they stay inside the root.
func (p *Store) ensureDir(name string) error {
	info, err := p.fs.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		if err := p.fs.Mkdir(name, directoryPerm); err != nil && !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("create plan directory: %w", err)
		}
		info, err = p.fs.Lstat(name)
	}
	if err != nil {
		return fmt.Errorf("inspect plan directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("plan path %q is not a non-symlink directory", name)
	}
	// Flush existing parents too: another publisher may have created the child
	// but not yet made that entry durable.
	if err := p.fs.SyncDir(filepath.Dir(name)); err != nil {
		return fmt.Errorf("sync plan parent: %w", err)
	}
	return nil
}

// Read verifies confinement, file type, byte count, and digest before returning.
func (p *Store) Read(ctx context.Context, blob Blob) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	content, err := p.readVerified(blob)
	if err != nil {
		return nil, &IntegrityError{Path: blob.RelativePath, Err: err}
	}
	return content, nil
}

// readVerified checks every path component without following symlinks first.
// Root operations independently enforce confinement if the tree changes.
func (p *Store) readVerified(blob Blob) ([]byte, error) {
	parts := strings.Split(blob.RelativePath, "/")
	if len(parts) != 3 || !validComponent(parts[0]) || !validComponent(parts[1]) || !validFilename(parts[2]) {
		return nil, errors.New("invalid blob path")
	}
	if blob.Bytes < 0 || blob.Bytes > MaxContentBytes {
		return nil, errors.New("invalid retained byte count")
	}
	path := ""
	for i, part := range parts {
		path = filepath.Join(path, part)
		info, err := p.fs.Lstat(path)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("symlink in blob path")
		}
		if i < 2 && !info.IsDir() {
			return nil, errors.New("invalid blob directory")
		}
		if i == 2 && !info.Mode().IsRegular() {
			return nil, errors.New("blob is not a regular file")
		}
	}
	f, err := p.fs.OpenFile(blob.RelativePath, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	// A corrupt oversized file cannot force an unbounded allocation.
	content, readErr := io.ReadAll(io.LimitReader(f, MaxContentBytes+1))
	if err := errors.Join(readErr, f.Close()); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(content)
	if int64(len(content)) != blob.Bytes || hex.EncodeToString(sum[:]) != blob.SHA256 {
		return nil, errors.New("retained size or SHA-256 mismatch")
	}
	return content, nil
}

// validComponent permits the repository generated-ID alphabet, not path syntax.
func validComponent(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, c := range value {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.':
		default:
			return false
		}
	}
	return true
}

// validFilename requires the publication layout rather than any relative path.
func validFilename(name string) bool {
	if !strings.HasSuffix(name, ".md") {
		return false
	}
	revision, id, ok := strings.Cut(strings.TrimSuffix(name, ".md"), "-")
	n, err := strconv.ParseInt(revision, 10, 64)
	return ok && err == nil && n > 0 && strconv.FormatInt(n, 10) == revision && validComponent(id)
}

// writeAll treats even a nil-error short write as a failed publication.
func writeAll(w io.Writer, content []byte) error {
	n, err := w.Write(content)
	if err != nil {
		return err
	}
	if n != len(content) {
		return io.ErrShortWrite
	}
	return nil
}

// validateUpload runs before any filesystem mutation. Identifiers are generated
// by the server; client titles, source names, and frontmatter are never inputs.
func validateUpload(projectID, planID string, revision int64, content []byte) error {
	for _, id := range []string{projectID, planID} {
		if !validComponent(id) {
			return fmt.Errorf("invalid plan storage identifier %q", id)
		}
	}
	if revision <= 0 {
		return errors.New("revision must be positive")
	}
	if len(content) > MaxContentBytes {
		return fmt.Errorf("plan content exceeds %d bytes", MaxContentBytes)
	}
	if !utf8.Valid(content) {
		return errors.New("plan content must be UTF-8")
	}
	return nil
}
