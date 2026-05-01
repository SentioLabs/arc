// Legacy share keyring import — one-shot migration from ~/.arc/shares.json
// into the shares table. Runs at arc-server startup after schema migrations.
// Idempotent: skips if the shares table already has rows. On successful
// import, renames the JSON file to shares.json.bak so the cutover is visible.
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
//   - if shares table already has rows: returns (0, nil) without touching the file
//   - on success: renames path to path+".bak" so subsequent startups treat it as already-imported
//   - on parse error: returns (0, err) and does not modify the file or database
func (s *Server) ImportLegacySharesJSON(ctx context.Context, path string) (int, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read legacy shares.json: %w", err)
	}

	existing, err := s.store.ListShares(ctx)
	if err != nil {
		return 0, fmt.Errorf("list shares: %w", err)
	}
	if len(existing) > 0 {
		return 0, nil
	}

	var lf legacyFile
	if err := json.Unmarshal(data, &lf); err != nil {
		return 0, fmt.Errorf("parse legacy shares.json: %w", err)
	}

	count := 0
	for _, ls := range lf.Shares {
		share := &types.Share{
			ID:        ls.ID,
			Kind:      types.ShareKind(ls.Kind),
			URL:       ls.URL,
			KeyB64Url: ls.KeyB64Url,
			EditToken: ls.EditToken,
			PlanFile:  ls.PlanFile,
			CreatedAt: ls.CreatedAt,
		}
		if err := s.store.UpsertShare(ctx, share); err != nil {
			return count, fmt.Errorf("upsert legacy share %s: %w", ls.ID, err)
		}
		count++
	}

	if err := os.Rename(path, path+".bak"); err != nil {
		return count, fmt.Errorf("rename legacy shares.json to .bak: %w", err)
	}

	return count, nil
}
