//go:build unix

package config

import (
	"os"

	"golang.org/x/sys/unix"
)

// flock locks belong to the open file, not the process, so two opens in one
// process exclude each other just as two processes do.
func lockFile(file *os.File) error {
	for {
		err := unix.Flock(int(file.Fd()), unix.LOCK_EX)
		if err != unix.EINTR {
			return err
		}
	}
}

func unlockFile(file *os.File) error { return unix.Flock(int(file.Fd()), unix.LOCK_UN) }
