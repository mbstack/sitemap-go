package sitemap

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

func TestErrInvalidConfig_Wrappable(t *testing.T) {
	err := fmt.Errorf("DataDir is required: %w", ErrInvalidConfig)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatal("expected wrap to preserve ErrInvalidConfig")
	}
}

func TestErrFetch_Wrappable(t *testing.T) {
	netErr := errors.New("dial tcp: no such host")
	err := fmt.Errorf("GET %s: %w", "https://x.example/robots.txt", fmt.Errorf("%w: %v", ErrFetch, netErr))
	if !errors.Is(err, ErrFetch) {
		t.Fatal("expected wrap to preserve ErrFetch")
	}
}
