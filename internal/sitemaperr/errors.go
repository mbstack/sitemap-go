// Package sitemaperr holds the sentinel errors for the
// sitemap-go library. They live in internal/ so external
// importers see them only via the public package re-exports
// (see package sitemap's errors.go). Putting them here also
// avoids an import cycle with sub-packages like config/.
package sitemaperr

import (
	"errors"
	"fmt"
)

// Sentinel errors. Wrap with fmt.Errorf and %w to add context.
var (
	// ErrInvalidConfig is returned when Config.Validate fails.
	// Callers should fix the Config and retry.
	ErrInvalidConfig = errors.New("sitemap: invalid config")

	// ErrNoSitemap is returned when robots.txt has no Sitemap
	// entries, or the site's sitemapindex was empty.
	ErrNoSitemap = errors.New("sitemap: no sitemap entries found")

	// ErrSiteNotFound is returned by the store when a Site row is
	// expected but not present.
	ErrSiteNotFound = errors.New("sitemap: site not found")

	// ErrFetch is the root of any HTTP / network failure. Concrete
	// failures (DNS, TLS, 4xx, 5xx, timeout) wrap this sentinel.
	ErrFetch = errors.New("sitemap: fetch failed")
)

// Wrap returns a wrapped error with the given operation label as
// a prefix. It is a thin convenience over
// fmt.Errorf("%s: %w", op, err) and is the standard way to add
// context inside the library.
//
//	return nil, sitemaperr.Wrap("robots", err)
func Wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}
