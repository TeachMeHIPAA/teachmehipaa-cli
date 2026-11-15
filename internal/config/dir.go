package config

import (
	"fmt"
	"path/filepath"
)

// DefaultDir is $XDG_CONFIG_HOME/tmh, or ~/.config/tmh, on every OS, as the
// Stripe CLI does: os.UserConfigDir would hide it under Library/Application
// Support on macOS and %AppData% on Windows, where nobody looks for it.
func DefaultDir(lookupEnv func(string) (string, bool), userHomeDir func() (string, error)) (string, error) {
	// The XDG spec says to ignore a relative path.
	if xdg, ok := lookupEnv("XDG_CONFIG_HOME"); ok && filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "tmh"), nil
	}
	home, err := userHomeDir()
	if err != nil {
		return "", fmt.Errorf("no home directory for the config: %w", err)
	}
	return filepath.Join(home, ".config", "tmh"), nil
}
