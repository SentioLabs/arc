// Package sharesconfig provides backward-compatible access to the user's
// share keyring. As of v0.next, the keyring is stored in arc-server's
// SQLite database (data.db); this package wraps the /api/v1/shares HTTP
// endpoints to preserve the existing public API used by cmd/arc/share.go.
package sharesconfig

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/types"
)

// ErrShareNotFound is returned by Find when no share matches the given ID.
// Preserved for backward compatibility with existing callers.
var ErrShareNotFound = errors.New("share not found")

// Share is the public representation of a keyring entry. Field names
// match the legacy shares.json on-disk format, so callers built against
// the JSON-backed implementation continue to compile.
type Share struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"` // "local" | "shared"
	URL       string    `json:"url"`
	KeyB64Url string    `json:"key_b64url"`
	EditToken string    `json:"edit_token"`
	PlanFile  string    `json:"plan_file,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// File preserves the legacy shape that callers iterate over.
type File struct {
	Shares []Share `json:"shares"`
}

// Client is the minimal HTTP client surface this package uses.
// The real implementation is internal/client.Client; tests inject fakes.
type Client interface {
	ListShares() ([]*types.Share, error)
	GetShare(id string) (*types.Share, error)
	UpsertShare(share *types.Share) (*types.Share, error)
	DeleteShare(id string) error
}

// clientFactory is injected at process startup (typically from cmd/arc/main.go).
// Tests use SetClientFactory to inject fakes.
var clientFactory func() (Client, error)

// SetClientFactory installs the function used to obtain the HTTP client.
// Must be called before any sharesconfig package function is invoked.
func SetClientFactory(fn func() (Client, error)) {
	clientFactory = fn
}

// getClient is the package-internal accessor; errors clearly if the factory
// hasn't been set, so misuse is loud.
func getClient() (Client, error) {
	if clientFactory == nil {
		return nil, errors.New("sharesconfig: client factory not initialized: call SetClientFactory in main")
	}
	return clientFactory()
}

// LegacyPath returns the path to the legacy ~/.arc/shares.json file.
// Used by the server-side import logic to find the file at startup.
// The CLI no longer reads this path.
func LegacyPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".arc", "shares.json"), nil
}

// Load returns all keyring entries, matching the legacy contract.
func Load() (*File, error) {
	c, err := getClient()
	if err != nil {
		return nil, err
	}
	remote, err := c.ListShares()
	if err != nil {
		return nil, err
	}
	f := &File{Shares: make([]Share, 0, len(remote))}
	for _, r := range remote {
		f.Shares = append(f.Shares, fromTypes(r))
	}
	return f, nil
}

// Add upserts a Share into the keyring.
func Add(s Share) error {
	c, err := getClient()
	if err != nil {
		return err
	}
	_, err = c.UpsertShare(toTypes(s))
	return err
}

// Find returns the keyring entry for the given share ID, or
// ErrShareNotFound if no entry matches.
func Find(id string) (*Share, error) {
	c, err := getClient()
	if err != nil {
		return nil, err
	}
	remote, err := c.GetShare(id)
	if err != nil {
		if errors.Is(err, client.ErrShareNotFound) {
			return nil, ErrShareNotFound
		}
		return nil, err
	}
	s := fromTypes(remote)
	return &s, nil
}

// Remove deletes the keyring entry for the given ID. No-op if missing.
func Remove(id string) error {
	c, err := getClient()
	if err != nil {
		return err
	}
	return c.DeleteShare(id)
}

func toTypes(s Share) *types.Share {
	return &types.Share{
		ID:        s.ID,
		Kind:      types.ShareKind(s.Kind),
		URL:       s.URL,
		KeyB64Url: s.KeyB64Url,
		EditToken: s.EditToken,
		PlanFile:  s.PlanFile,
		CreatedAt: s.CreatedAt,
	}
}

func fromTypes(t *types.Share) Share {
	return Share{
		ID:        t.ID,
		Kind:      string(t.Kind),
		URL:       t.URL,
		KeyB64Url: t.KeyB64Url,
		EditToken: t.EditToken,
		PlanFile:  t.PlanFile,
		CreatedAt: t.CreatedAt,
	}
}
