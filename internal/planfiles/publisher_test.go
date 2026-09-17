package planfiles //nolint:testpackage // failure injection exercises private filesystem primitives

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestPublisher(t *testing.T, root string) *Store {
	t.Helper()
	p, err := New(root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, p.Close()) })
	return p
}

func TestPublishedContentSurvivesReopen(t *testing.T) {
	for _, want := range [][]byte{[]byte("# Plan\r\nExact bytes\n☃\n"), {}} {
		root := t.TempDir()
		p := newTestPublisher(t, root)
		blob, err := p.Publish(context.Background(), "ws-test", "plan.test", 1, want)
		require.NoError(t, err)
		sum := sha256.Sum256(want)
		require.Equal(t, hex.EncodeToString(sum[:]), blob.SHA256)
		require.EqualValues(t, len(want), blob.Bytes)
		got, err := newTestPublisher(t, root).Read(context.Background(), blob)
		require.NoError(t, err)
		require.Equal(t, want, got)
		other, err := p.Publish(context.Background(), "ws-test", "plan.test", 1, want)
		require.NoError(t, err)
		require.NotEqual(t, blob.RelativePath, other.RelativePath)
	}
}

func TestPublishRejectsInvalidInputBeforeCreatingDirectories(t *testing.T) {
	for _, tc := range []struct {
		name, project, plan string
		revision            int64
		content             []byte
	}{
		{"utf8", "ws-test", "plan.test", 1, []byte{0xff}},
		{"limit", "ws-test", "plan.test", 1, bytes.Repeat([]byte("x"), 10*1024*1024+1)},
		{"revision", "ws-test", "plan.test", 0, nil},
		{"negative", "ws-test", "plan.test", -1, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			p := newTestPublisher(t, root)
			blob, err := p.Publish(context.Background(), tc.project, tc.plan, tc.revision, tc.content)
			require.Error(t, err)
			require.Empty(t, blob)
			entries, err := os.ReadDir(root)
			require.NoError(t, err)
			require.Len(t, entries, 1)
			require.Equal(t, lockName, entries[0].Name())
		})
	}
	for _, bad := range []string{"", ".", "..", "../escape", "/tmp/escape", "a/b", `a\b`, "a\x00b", "a b"} {
		for _, project := range []bool{true, false} {
			root := t.TempDir()
			p := newTestPublisher(t, root)
			pid, id := "ws-test", "plan.test"
			if project {
				pid = bad
			} else {
				id = bad
			}
			blob, err := p.Publish(context.Background(), pid, id, 1, nil)
			require.Error(t, err, bad)
			require.Empty(t, blob)
			entries, err := os.ReadDir(root)
			require.NoError(t, err)
			require.Len(t, entries, 1)
			require.Equal(t, lockName, entries[0].Name())
		}
	}
}

func TestPublishLimit(t *testing.T) {
	p := newTestPublisher(t, t.TempDir())
	want := bytes.Repeat([]byte("x"), 10*1024*1024)
	blob, err := p.Publish(context.Background(), "ws-test", "plan.test", 1, want)
	require.NoError(t, err)
	got, err := p.Read(context.Background(), blob)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestRejectsSymlinkComponents(t *testing.T) {
	for _, component := range []string{"ws-test", "ws-test/plan.test"} {
		t.Run(component, func(t *testing.T) {
			root := t.TempDir()
			p := newTestPublisher(t, root)
			require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, component)), 0o700))
			require.NoError(t, os.Mkdir(filepath.Join(root, "other"), 0o700))
			target := "other"
			if strings.Contains(component, "/") {
				target = "../other"
			}
			require.NoError(t, os.Symlink(target, filepath.Join(root, component)))
			blob, err := p.Publish(context.Background(), "ws-test", "plan.test", 1, nil)
			require.Error(t, err)
			require.Empty(t, blob)
			entries, err := os.ReadDir(filepath.Join(root, "other"))
			require.NoError(t, err)
			require.Empty(t, entries)
		})
	}
}

func TestReadIntegrity(t *testing.T) {
	for _, mode := range []string{
		"tamper", "length", "delete", "symlink", "project-symlink", "plan-symlink", "traversal",
	} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			p := newTestPublisher(t, root)
			blob, err := p.Publish(context.Background(), "ws-test", "plan.test", 1, []byte("original"))
			require.NoError(t, err)
			path := filepath.Join(root, blob.RelativePath)
			switch mode {
			case "tamper":
				require.NoError(t, os.WriteFile(path, []byte("modified"), 0o600))
			case "length":
				blob.Bytes++
			case "delete":
				require.NoError(t, os.Remove(path))
			case "symlink":
				require.NoError(t, os.Rename(path, path+".copy"))
				require.NoError(t, os.Symlink(filepath.Base(path)+".copy", path))
			case "project-symlink", "plan-symlink":
				dir := filepath.Join(root, "ws-test")
				if mode == "plan-symlink" {
					dir = filepath.Dir(path)
				}
				require.NoError(t, os.Rename(dir, dir+".copy"))
				require.NoError(t, os.Symlink(filepath.Base(dir)+".copy", dir))
			case "traversal":
				blob.RelativePath = "../" + blob.RelativePath
			}
			got, err := p.Read(context.Background(), blob)
			var integrity *IntegrityError
			require.ErrorAs(t, err, &integrity)
			require.Nil(t, got)
		})
	}
}

func TestUnavailableRoot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file")
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	_, err := New(filepath.Join(path, "plans"))
	require.Error(t, err)
	_, err = New("relative")
	require.Error(t, err)
	p := newTestPublisher(t, filepath.Join(root, "new", "plans"))
	_, err = p.Publish(context.Background(), "ws-test", "plan.test", 1, nil)
	require.NoError(t, err)
	require.NoError(t, os.Chmod(filepath.Join(root, "new", "plans"), 0o500))
	t.Cleanup(func() { require.NoError(t, os.Chmod(filepath.Join(root, "new", "plans"), 0o700)) })
	_, err = New(filepath.Join(root, "new", "plans"))
	require.Error(t, err)
}

var errInjected = errors.New("injected filesystem failure")

type failingFS struct {
	fileSystem
	fail  string
	calls []string
}

func (f *failingFS) OpenFile(name string, flag int, perm os.FileMode) (file, error) {
	if f.fail == "open" {
		return nil, errInjected
	}
	v, err := f.fileSystem.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return &failingFile{file: v, fs: f}, nil
}

func (f *failingFS) Mkdir(name string, perm os.FileMode) error {
	if f.fail == "mkdir" {
		return errInjected
	}
	return f.fileSystem.Mkdir(name, perm)
}

func (f *failingFS) Rename(from, to string) error {
	f.calls = append(f.calls, "rename")
	if f.fail == "rename" {
		return errInjected
	}
	return f.fileSystem.Rename(from, to)
}

func (f *failingFS) SyncDir(name string) error {
	f.calls = append(f.calls, "dir-sync:"+name)
	if f.fail == "parent-sync" || (f.fail == "dir-sync" && strings.Contains(name, "plan.test")) {
		return errInjected
	}
	return f.fileSystem.SyncDir(name)
}

type failingFile struct {
	file
	fs *failingFS
}

func (f *failingFile) Write(b []byte) (int, error) {
	f.fs.calls = append(f.fs.calls, "write")
	if f.fs.fail == "write" {
		return 0, errInjected
	}
	if f.fs.fail == "short-write" {
		return len(b) - 1, nil
	}
	return f.file.Write(b)
}

func (f *failingFile) Sync() error {
	f.fs.calls = append(f.fs.calls, "file-sync")
	if f.fs.fail == "file-sync" {
		return errInjected
	}
	return f.file.Sync()
}

func (f *failingFile) Close() error {
	f.fs.calls = append(f.fs.calls, "close")
	err := f.file.Close()
	if f.fs.fail == "close" {
		return errInjected
	}
	return err
}

func TestPublicationFailuresNeverReturnBlob(t *testing.T) {
	for _, stage := range []string{
		"mkdir", "parent-sync", "open", "write", "short-write", "file-sync", "close", "rename", "dir-sync",
	} {
		t.Run(stage, func(t *testing.T) {
			root := t.TempDir()
			p := newTestPublisher(t, root)
			fs := &failingFS{fileSystem: p.fs, fail: stage}
			p.fs = fs
			blob, err := p.Publish(context.Background(), "ws-test", "plan.test", 1, []byte("exact bytes"))
			if stage == "short-write" {
				require.ErrorIs(t, err, io.ErrShortWrite)
			} else {
				require.ErrorIs(t, err, errInjected)
			}
			require.Empty(t, blob)
			require.NoError(t, filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				require.NoError(t, err)
				if !d.IsDir() && d.Name() != lockName {
					require.True(t, strings.HasSuffix(path, ".tmp") || strings.HasSuffix(path, ".md"))
					require.Zero(t, d.Type()&os.ModeSymlink)
				}
				return nil
			}))
		})
	}
}

func TestPublicationFlushOrder(t *testing.T) {
	p := newTestPublisher(t, t.TempDir())
	fs := &failingFS{fileSystem: p.fs}
	p.fs = fs
	_, err := p.Publish(context.Background(), "ws-test", "plan.test", 1, []byte("content"))
	require.NoError(t, err)
	require.Equal(t, []string{
		"dir-sync:.", "dir-sync:ws-test", "write", "file-sync", "close", "rename", "dir-sync:ws-test/plan.test",
	}, fs.calls)
}

func TestPublishRejectsExistingAndSymlinkTargets(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		root := t.TempDir()
		p := newTestPublisher(t, root)
		require.NoError(t, os.MkdirAll(filepath.Join(root, "ws-test", "plan.test"), 0o700))
		target := filepath.Join(root, "ws-test", "plan.test", "1-fixed.md")
		if symlink {
			require.NoError(t, os.WriteFile(filepath.Join(root, "original"), []byte("keep"), 0o600))
			require.NoError(t, os.Symlink("../../original", target))
		} else {
			require.NoError(t, os.WriteFile(target, []byte("keep"), 0o600))
		}
		p.newID = func() string { return "fixed" }
		blob, err := p.Publish(context.Background(), "ws-test", "plan.test", 1, []byte("overwrite"))
		require.Error(t, err)
		require.Empty(t, blob)
		got, err := os.ReadFile(target)
		require.NoError(t, err)
		require.Equal(t, "keep", string(got))
	}
}
