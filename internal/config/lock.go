package config

import (
	"context"
	"os"
	"path/filepath"
)

// LockGrant holds an exclusive lock on the profile's Grant until unlock is
// called. Two tmh processes presenting the same Refresh token would revoke
// the Grant, so every exchange happens under it; it is a file lock because
// those processes share nothing else.
func (s Store) LockGrant(ctx context.Context, profile string) (unlock func(), err error) {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return nil, err
	}
	// The file is never removed: a process could still hold the lock on the
	// removed file while another locks a new one at the same path.
	file, err := os.OpenFile(filepath.Join(s.Dir, "."+profile+"-grant.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	locked := make(chan error, 1)
	go func() { locked <- lockFile(file) }()
	select {
	case err := <-locked:
		if err != nil {
			file.Close()
			return nil, err
		}
		return func() {
			_ = unlockFile(file)
			file.Close()
		}, nil
	case <-ctx.Done():
		// Closing the file releases the lock should it arrive later.
		go func() {
			<-locked
			file.Close()
		}()
		return nil, ctx.Err()
	}
}
