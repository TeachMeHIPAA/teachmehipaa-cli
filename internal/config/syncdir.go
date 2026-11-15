//go:build !windows

package config

import "os"

// syncDir makes a rename into dir survive a crash.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
