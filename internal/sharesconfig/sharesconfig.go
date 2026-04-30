// Package sharesconfig manages the registry of paste shares the user has
// created, stored at ~/.arc/shares.json with file mode 0600.
package sharesconfig

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// File permission bits for the shares registry. shares.json holds edit
// tokens, so it must not be world-readable.
const (
	dirMode  os.FileMode = 0o700
	fileMode os.FileMode = 0o600
)

// ErrShareNotFound is returned by Find when no share matches the given ID.
var ErrShareNotFound = errors.New("share not found")

// Share holds the metadata for a single paste share created by this machine.
type Share struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"` // "local" | "shared"
	URL       string    `json:"url"`
	KeyB64Url string    `json:"key_b64url"`
	EditToken string    `json:"edit_token"`
	PlanFile  string    `json:"plan_file,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// File is the top-level structure of ~/.arc/shares.json.
type File struct {
	Shares []Share `json:"shares"`
}

// defaultPath returns the path to ~/.arc/shares.json.
func defaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".arc", "shares.json"), nil
}

// Load reads the shares file from disk. Returns an empty File if the file does
// not exist yet.
func Load() (*File, error) {
	path, err := defaultPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &File{}, nil
	}
	if err != nil {
		return nil, err
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// Save writes the File to disk at ~/.arc/shares.json with mode 0600. The
// parent directory is created with mode 0700 if it does not exist.
func Save(f *File) error {
	path, err := defaultPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, fileMode)
}

// Add upserts a Share into the registry. If a share with the same ID already
// exists it is replaced; otherwise the share is appended.
func Add(s Share) error {
	f, err := Load()
	if err != nil {
		return err
	}
	for i, existing := range f.Shares {
		if existing.ID == s.ID {
			f.Shares[i] = s
			return Save(f)
		}
	}
	f.Shares = append(f.Shares, s)
	return Save(f)
}

// Find returns the Share with the given ID, or ErrShareNotFound if no entry
// matches. Callers may also use errors.Is(err, ErrShareNotFound) to branch.
func Find(id string) (*Share, error) {
	f, err := Load()
	if err != nil {
		return nil, err
	}
	for _, s := range f.Shares {
		if s.ID == id {
			return &s, nil
		}
	}
	return nil, ErrShareNotFound
}

// Remove deletes the share with the given ID from the registry. It is a no-op
// if the ID does not exist.
func Remove(id string) error {
	f, err := Load()
	if err != nil {
		return err
	}
	out := f.Shares[:0]
	for _, s := range f.Shares {
		if s.ID != id {
			out = append(out, s)
		}
	}
	f.Shares = out
	return Save(f)
}
