package api

import (
	"context"

	"github.com/sentiolabs/arc/internal/gitfs"
	"github.com/sentiolabs/arc/internal/types"
)

// resolveProjectForPath is the canonical server-side path-to-project resolver.
//
// Stages:
//  1. Match `path` exactly or against the longest registered ancestor via
//     store.ResolveProjectByPath (prefix-aware).
//  2. If (1) fails and `path` is inside a linked git worktree, retry (1)
//     against the main repository's working directory.
//
// Returns the matched workspace, or the underlying not-found error from
// the storage layer if no stage succeeds.
func (s *Server) resolveProjectForPath(ctx context.Context, path string) (*types.Workspace, error) {
	ws, err := s.store.ResolveProjectByPath(ctx, path)
	if err == nil {
		return ws, nil
	}

	mainRepo := gitfs.DetectMainRepo(path)
	if mainRepo == "" {
		return nil, err
	}

	if mainWs, retryErr := s.store.ResolveProjectByPath(ctx, mainRepo); retryErr == nil {
		return mainWs, nil
	}
	return nil, err
}
