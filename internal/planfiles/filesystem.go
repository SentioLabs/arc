package planfiles

import (
	"errors"
	"io"
	"os"
)

// Filesystem primitives remain replaceable in package tests so failure ordering
// can be exercised against real temporary files without a live content root.
type file interface {
	io.Reader
	io.Writer
	Sync() error
	Close() error
}
type fileSystem interface {
	Lstat(string) (os.FileInfo, error)
	Mkdir(string, os.FileMode) error
	OpenFile(string, int, os.FileMode) (file, error)
	Rename(string, string) error
	SyncDir(string) error
}
type rootFS struct{ *os.Root }

func (r rootFS) OpenFile(name string, flags int, perm os.FileMode) (file, error) {
	return r.Root.OpenFile(name, flags, perm)
}

func (r rootFS) SyncDir(name string) error {
	dir, err := r.Open(name)
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}
