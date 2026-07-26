// Package sitemap provides Scanner, the central type that fetches
// a site's robots.txt, extracts its sitemap index(es), walks them,
// and persists the discovered URLs to a Store.
//
// All public methods return error. Sentinel errors are re-exported
// from internal/sitemaperr so external callers can match them
// with errors.Is:
//
//	if errors.Is(err, sitemap.ErrNoSitemap) { ... }
package sitemap

import "github.com/mbstack/sitemap-go/internal/sitemaperr"

// Re-exported sentinel errors. See internal/sitemaperr for
// definitions.
var (
	ErrInvalidConfig = sitemaperr.ErrInvalidConfig
	ErrNoSitemap     = sitemaperr.ErrNoSitemap
	ErrSiteNotFound  = sitemaperr.ErrSiteNotFound
	ErrFetch         = sitemaperr.ErrFetch
)

// Wrap returns a wrapped error with the given operation label.
// Convenience over fmt.Errorf("%s: %w", op, err). See
// internal/sitemaperr.Wrap.
func Wrap(op string, err error) error {
	return sitemaperr.Wrap(op, err)
}
