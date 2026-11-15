//go:build windows

package config

// syncDir does nothing: Windows can't open a directory for syncing, and
// NTFS journals the rename itself.
func syncDir(string) error { return nil }
