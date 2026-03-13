package project

import (
	"github.com/sentiolabs/arc/internal/core"
)

// DefaultArcHome returns the default arc home directory (~/.arc).
// Delegates to core.DefaultArcHome; kept for backward compatibility.
func DefaultArcHome() string { return core.DefaultArcHome() }

// NormalizePath resolves symlinks and returns the canonical absolute path.
// Delegates to core.NormalizePath; kept for backward compatibility.
func NormalizePath(path string) string { return core.NormalizePath(path) }

// NormalizePathPair returns both the absolute path and the symlink-resolved path.
// Delegates to core.NormalizePathPair; kept for backward compatibility.
func NormalizePathPair(path string) (absPath, resolvedPath string) {
	return core.NormalizePathPair(path)
}
