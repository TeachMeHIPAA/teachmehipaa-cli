// Package clock supplies the current time, so token expiry is testable.
package clock

import "time"

type Clock interface {
	Now() time.Time
}

// System is the wall clock.
func System() Clock { return systemClock{} }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }
