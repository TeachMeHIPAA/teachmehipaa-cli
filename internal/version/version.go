// Package version holds the build's version, set by the release build.
package version

// These are overridden with -ldflags "-X .../internal/version.Version=1.2.3"
// and likewise for Commit and Date.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Info describes one build of tmh.
type Info struct {
	Version string
	Commit  string
	// Date is when the build was made, RFC 3339.
	Date string
}

// Current is this binary's build.
func Current() Info { return Info{Version: Version, Commit: Commit, Date: Date} }
