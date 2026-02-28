// Package version provides build version information.
package version //nolint:revive // version is a clear, descriptive package name

import (
	"fmt"
	"runtime"
)

// commitHashPrefixLen is the number of characters to show from the git commit hash.
const commitHashPrefixLen = 7

// Build information, set via ldflags.
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// Info returns formatted version information.
func Info() string {
	return fmt.Sprintf("arc %s (%s) built %s with %s",
		Version, GitCommit[:min(commitHashPrefixLen, len(GitCommit))], BuildDate, runtime.Version())
}

// Short returns just the version string.
func Short() string {
	return Version
}
