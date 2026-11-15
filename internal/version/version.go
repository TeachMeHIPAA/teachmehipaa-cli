// Package version holds the build's version, set by the release build.
package version

// Version is overridden with -ldflags "-X .../internal/version.Version=v1.2.3".
var Version = "dev"
