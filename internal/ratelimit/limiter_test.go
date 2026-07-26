package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func TestDuration_InBounds(t *testing.T) {
	const N = 1000
	min := 1 * time.Microsecond
	max := 1 * time.Millisecond
	l := Limiter{Min: min, Max: max}
	for i := 0; i < N; i++ {
		got := l.Duration()
		if got < min || got > max {
			t.Fatalf("duration %s out of [%s,%s]", got, min, max)
		}
	}
}

func TestDuration_ExactWhenEqual(t *testing.T) {
	l := Limiter{Min: 5 * time.Millisecond, Max: 5 * time.Millisecond}
	if got := l.Duration(); got != 5*time.Millisecond {
		t.Fatalf("want 5ms exact, got %s", got)
	}
}

func TestDuration_ZeroWhenBothZero(t *testing.T) {
	l := Limiter{}
	if got := l.Duration(); got != 0 {
		t.Fatalf("zero limiter should be 0, got %s", got)
	}
}

func TestDuration_SwapsWhenMinGreater(t *testing.T) {
	l := Limiter{Min: 100 * time.Microsecond, Max: 10 * time.Microsecond}
	// 1000 draws, every one must be in the swapped range.
	for i := 0; i < 1000; i++ {
		got := l.Duration()
		if got < 10*time.Microsecond || got > 100*time.Microsecond {
			t.Fatalf("expected swap, got %s out of [10us,100us]", got)
		}
	}
}

func TestDuration_ClampsNegativeMin(t *testing.T) {
	l := Limiter{Min: -5 * time.Millisecond, Max: 0}
	for i := 0; i < 100; i++ {
		if got := l.Duration(); got != 0 {
			t.Fatalf("clamped duration: want 0, got %s", got)
		}
	}
}

func TestWait_NoPanicConcurrent(t *testing.T) {
	l := Limiter{Min: 0, Max: 100 * time.Microsecond}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Wait()
		}()
	}
	wg.Wait()
}
