// Package ratelimit provides a Limiter that sleeps a uniformly
// random duration in [Min, Max] each time Wait is called.
package ratelimit

import (
	"math/rand/v2"
	"time"
)

// Limiter sleeps a uniformly random duration in [Min, Max]
// each time Wait is called. The zero value is a usable no-op
// (both fields zero → no sleep).
//
// The type is safe for concurrent use.
type Limiter struct {
	// Min and Max bound the random sleep. If Min > Max the
	// values are swapped. If Min < 0 it is clamped to 0.
	Min, Max time.Duration
}

// Wait sleeps for a random duration in [Min, Max]. When Min and
// Max are both zero Wait returns immediately.
func (l Limiter) Wait() {
	min, max := l.Min, l.Max
	if min > max {
		min, max = max, min
	}
	if min < 0 {
		min = 0
	}
	if max <= min {
		// Either exact, or both zero — single sleep, no RNG.
		if min > 0 {
			time.Sleep(min)
		}
		return
	}
	delta := int64(max - min)
	// math/rand/v2: top exclusive. Add 1 to make max inclusive.
	off := rand.Int64N(delta + 1)
	time.Sleep(min + time.Duration(off))
}

// Duration returns a uniformly random duration in [Min, Max]
// without sleeping. Useful for tests and instrumentation.
func (l Limiter) Duration() time.Duration {
	min, max := l.Min, l.Max
	if min > max {
		min, max = max, min
	}
	if min < 0 {
		min = 0
	}
	if max <= min {
		return min
	}
	delta := int64(max - min)
	return min + time.Duration(rand.Int64N(delta+1))
}
