// Package version provides build version information.
package version

import (
	"fmt"
	"runtime"
)

// Build information, set via ldflags.
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// Info returns formatted version information.
func Info() string {
	return fmt.Sprintf("arc %s (%s) built %s with %s",
		Version, GitCommit[:min(7, len(GitCommit))], BuildDate, runtime.Version())
}

// Short returns just the version string.
func Short() string {
	return Version
}
