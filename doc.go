// Package sitemap provides Scanner, the central type that
// fetches a site's robots.txt, extracts its sitemap
// index(es), walks them, and persists the discovered URLs to
// a Store.
//
// The library is composed of small, single-purpose packages:
//
//   - config  — runtime configuration with validated defaults
//   - logger  — zerolog wrapper
//   - sitemapxml — pure XML parsers (no I/O)
//   - store   — persistence contract (Store interface) and gorm
//     models
//   - store/sqlite — default gorm-backed SQLite Store
//   - internal/httpx     — http.Client builder
//   - internal/ratelimit — bounded random delay
//   - internal/sitemaperr — sentinel errors
//
// The library never panics on recoverable errors. Every public
// method returns error. Sentinel errors in this package
// (ErrInvalidConfig, ErrNoSitemap, ErrSiteNotFound, ErrFetch)
// can be matched with errors.Is.
//
// # Quickstart
//
//	cfg := config.DefaultConfig()
//	cfg.DataDir = "./data"
//	cfg.Logger = logger.New(logger.Options{Level: "info", Pretty: true})
//
//	s, err := sitemap.NewScanner(context.Background(), cfg)
//	if err != nil { return err }
//	defer s.Close()
//
//	if err := s.ScanSite(ctx, "https://example.com"); err != nil { return err }
//	if err := s.ScanSitemapIndex(ctx, 100); err != nil { return err }
//
// See the cmd/sitemap CLI for a runnable example.
package sitemap
