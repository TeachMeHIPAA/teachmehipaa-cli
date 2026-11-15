// Package clock supplies the current time and waits, so token expiry and
// retry backoff are testable without real delays.
package clock

import (
	"context"
	"time"
)

// Clock is where tmh reads the time and waits.
type Clock interface {
	// Now is the current time.
	Now() time.Time
	// Sleep waits for d, or returns the context's error if it ends first.
	Sleep(ctx context.Context, d time.Duration) error
}

// System is the wall clock.
func System() Clock { return systemClock{} }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func (systemClock) Sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
