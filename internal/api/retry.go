package api

import (
	"errors"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	initialRetryDelay = 500 * time.Millisecond
	maxRetryDelay     = 5 * time.Second
	maxRetryAfter     = 60 * time.Second
)

// retryDelay honors `Retry-After` up to 60 s, and otherwise backs off.
func retryDelay(err error, retry int, now time.Time) time.Duration {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		if wait, ok := retryAfter(apiErr.Header.Get("Retry-After"), now); ok {
			return min(wait, maxRetryAfter)
		}
	}
	return backoff(retry)
}

// backoff is Stripe's schedule, as in the SDKs: the ceiling doubles from
// 0.5 s to 5 s, jitter picks 50–100 % of it, and nothing waits under 0.5 s.
// The shift stops growing long past the cap so it can't overflow.
func backoff(retry int) time.Duration {
	ceiling := min(initialRetryDelay<<min(retry-1, 10), maxRetryDelay)
	jittered := time.Duration(float64(ceiling) * (0.5 + rand.Float64()*0.5))
	return max(initialRetryDelay, jittered)
}

// retryAfter reads delay-seconds or an HTTP-date (RFC 9110 §10.2.3);
// anything else counts as absent.
func retryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.ParseUint(value, 10, 32); err == nil {
		return time.Duration(seconds) * time.Second, true
	}
	date, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	seconds := math.Ceil(date.Sub(now).Seconds())
	return time.Duration(max(0, seconds)) * time.Second, true
}
