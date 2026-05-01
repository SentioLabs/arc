// Legacy share keyring import — one-shot migration from ~/.arc/shares.json
// into the shares table. Runs at arc-server startup after schema migrations.
// Idempotency comes from the file's presence: a successful import renames
// the JSON to shares.json.bak, so subsequent startups simply find no file
// to read and return early.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/sentiolabs/arc/internal/types"
)

// legacyFile mirrors the on-disk format of ~/.arc/shares.json.
type legacyFile struct {
	Shares []legacyShare `json:"shares"`
}

type legacyShare struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	URL       string    `json:"url"`
	KeyB64Url string    `json:"key_b64url"`
	EditToken string    `json:"edit_token"`
	PlanFile  string    `json:"plan_file,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ImportLegacySharesJSON imports the legacy shares.json keyring into the
// shares table. Returns the number of records imported. Behavior:
//   - if path does not exist: returns (0, nil)
//   - on parse error: returns (0, err) and does not modify the file or database
//   - on per-share validation/constraint failure: rolls the whole batch back
//     atomically, leaves the JSON file untouched, and returns the error so
//     the next startup can retry after the user fixes the entry
//   - on success: renames path to path+".bak" so subsequent startups find no
//     file to read and return early
func (s *Server) ImportLegacySharesJSON(ctx context.Context, path string) (int, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read legacy shares.json: %w", err)
	}

	var lf legacyFile
	if err := json.Unmarshal(data, &lf); err != nil {
		return 0, fmt.Errorf("parse legacy shares.json: %w", err)
	}

	shares := make([]*types.Share, 0, len(lf.Shares))
	for _, ls := range lf.Shares {
		shares = append(shares, &types.Share{
			ID:        ls.ID,
			Kind:      types.ShareKind(ls.Kind),
			URL:       ls.URL,
			KeyB64Url: ls.KeyB64Url,
			EditToken: ls.EditToken,
			PlanFile:  ls.PlanFile,
			CreatedAt: ls.CreatedAt,
		})
	}

	if err := s.store.UpsertShares(ctx, shares); err != nil {
		return 0, fmt.Errorf("import legacy shares: %w", err)
	}

	if err := os.Rename(path, path+".bak"); err != nil {
		return len(shares), fmt.Errorf("rename legacy shares.json to .bak: %w", err)
	}

	return len(shares), nil
}
