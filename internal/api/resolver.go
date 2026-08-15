package api

import (
	"context"

	"github.com/sentiolabs/arc/internal/core"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/sentiolabs/arc/internal/vcs"
)

// resolveProjectForPath is the canonical server-side path-to-project resolver.
//
// Stages:
//  1. Match `path` exactly or against the longest registered ancestor via
//     store.ResolveProjectByPath (prefix-aware).
//  2. If (1) fails and `path` is inside a linked git worktree or a secondary
//     jj workspace, retry (1) against the main repository's working directory
//     (via vcs.DetectMainRepo, which canonicalizes the result).
//
// The incoming path is canonicalized (symlinks resolved) up front so it agrees
// with the canonical form under which workspaces are registered and with the
// canonical result of vcs.DetectMainRepo. Without this, a path reached through
// a symlinked ancestor (e.g. macOS's /var -> /private/var) never matches the
// stored form and stage 2 falls through to a spurious not-found.
//
// Returns the matched workspace, or the underlying not-found error from
// the storage layer if no stage succeeds.
func (s *Server) resolveProjectForPath(ctx context.Context, path string) (*types.Workspace, error) {
	path = core.NormalizePath(path)

	ws, err := s.store.ResolveProjectByPath(ctx, path)
	if err == nil {
		return ws, nil
	}

	mainRepo := vcs.DetectMainRepo(path)
	if mainRepo == "" {
		return nil, err
	}

	if mainWs, retryErr := s.store.ResolveProjectByPath(ctx, mainRepo); retryErr == nil {
		return mainWs, nil
	}
	return nil, err
}
