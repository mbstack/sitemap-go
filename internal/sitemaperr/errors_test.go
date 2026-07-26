package sitemaperr

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinels_AreDistinct(t *testing.T) {
	sentinels := []error{
		ErrInvalidConfig,
		ErrNoSitemap,
		ErrSiteNotFound,
		ErrFetch,
	}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Fatalf("sentinel %d (%v) wrongly matches sentinel %d (%v)", i, a, j, b)
			}
		}
	}
}

func TestWrap_PreservesSentinel(t *testing.T) {
	inner := fmt.Errorf("inner: %w", ErrNoSitemap)
	outer := Wrap("phase.test", inner)

	if !errors.Is(outer, ErrNoSitemap) {
		t.Fatalf("Wrap should preserve ErrNoSitemap, got %v", outer)
	}
	if got := outer.Error(); got == "" {
		t.Fatal("Wrap returned empty string")
	}
}

func TestWrap_NilReturnsNil(t *testing.T) {
	if got := Wrap("op", nil); got != nil {
		t.Fatalf("Wrap(nil) should return nil, got %v", got)
	}
}
