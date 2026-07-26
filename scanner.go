// Package sitemap: Scanner type and constructor.
package sitemap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/mbstack/sitemap-go/config"
	"github.com/mbstack/sitemap-go/internal/ratelimit"
	"github.com/mbstack/sitemap-go/internal/sitemaperr"
	"github.com/mbstack/sitemap-go/sitemapxml"
	"github.com/mbstack/sitemap-go/store"
	"github.com/mbstack/sitemap-go/store/sqlite"
	"github.com/mbstack/sitemap-go/types"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

// Scanner is the central type that walks a site's sitemap(s)
// and persists the discovered URLs to a Store. Construct with
// NewScanner; always call Close when done.
type Scanner struct {
	cfg   config.Config
	log   *zerolog.Logger
	store store.Store
	fetch Fetcher
	delay ratelimit.Limiter
}

// NewScanner constructs a Scanner. Returns an error wrapping
// ErrInvalidConfig when cfg.Validate fails. The Store and
// Fetcher are constructed from the defaults (sqlite.Store and
// CollyFetcher) when not supplied via Config.
func NewScanner(ctx context.Context, cfg config.Config) (*Scanner, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.EnsureDataDir(); err != nil {
		return nil, sitemaperr.Wrap("ensure data dir", err)
	}

	st, err := buildStore(cfg)
	if err != nil {
		return nil, err
	}

	f, err := NewCollyFetcher(cfg, cfg.Logger)
	if err != nil {
		_ = st.Close()
		return nil, err
	}

	return &Scanner{
		cfg:   cfg,
		log:   cfg.Logger,
		store: st,
		fetch: f,
		delay: ratelimit.Limiter{Min: cfg.MinDelay, Max: cfg.MaxDelay},
	}, nil
}

// buildStore opens a sqlite-backed Store at cfg.DBPath and
// takes ownership of the underlying *sql.DB so Scanner.Close
// can release the file lock. The function is small but
// isolated for tests.
func buildStore(cfg config.Config) (store.Store, error) {
	db, err := sqlite.Open(cfg.DBPath())
	if err != nil {
		return nil, sitemaperr.Wrap("open sqlite", err)
	}
	if err := sqlite.Migrate(db); err != nil {
		return nil, sitemaperr.Wrap("migrate", err)
	}
	return sqlite.NewOwned(db), nil
}

// Close releases the store. Safe to call multiple times.
func (s *Scanner) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.Close()
}

// ScanSite fetches the robots.txt for siteURL, extracts the
// declared sitemapindex URLs, fetches and parses each
// sitemapindex, and persists the discovered sitemap rows.
//
// The site row is upserted. Returns ErrNoSitemap if robots.txt
// has no Sitemap entries or the discovered list is empty.
func (s *Scanner) ScanSite(ctx context.Context, siteURL string) error {
	if s == nil {
		return errors.New("scanner: nil receiver")
	}
	robots, err := GetSitemapURLs(ctx, siteURL, s.cfg)
	if err != nil {
		return err
	}

	// Fetch each sitemapindex, parse the leaf URLs, and
	// collect them.
	leafURLs := make([]string, 0, len(robots.Sitemaps))
	for _, smURL := range robots.Sitemaps {
		s.delay.Wait()
		if err := ctx.Err(); err != nil {
			return err
		}
		body, err := GetOK(ctx, s.fetch, smURL)
		if err != nil {
			s.log.Warn().Err(err).Str("url", smURL).Msg("fetch sitemapindex failed, skipping")
			continue
		}
		entries, err := sitemapxml.ParseSitemapIndex(bytesReader(body))
		if err != nil {
			s.log.Warn().Err(err).Str("url", smURL).Msg("parse sitemapindex failed, skipping")
			continue
		}
		for _, e := range entries {
			leafURLs = append(leafURLs, e.URL)
		}
	}
	if len(leafURLs) == 0 {
		return sitemaperr.ErrNoSitemap
	}

	// Persist the site row with the discovered sitemapindex
	// URLs (the parents). The leaves extracted from each
	// sitemapindex are persisted below as the rows that
	// ScanSitemapIndex will walk.
	sitemapsJSON, err := json.Marshal(robots.Sitemaps)
	if err != nil {
		return sitemaperr.Wrap("marshal sitemaps", err)
	}
	site := &store.Site{
		Domain:   domainOf(siteURL),
		URL:      siteURL,
		Sitemaps: string(sitemapsJSON),
	}
	if err := s.store.SaveSite(ctx, site); err != nil {
		return sitemaperr.Wrap("save site", err)
	}

	// Persist one row per leaf <sitemap><loc> found inside
	// the declared sitemapindex files. ScanSitemapIndex
	// walks these rows.
	indexes := make([]store.SitemapIndex, 0, len(leafURLs))
	for _, u := range leafURLs {
		indexes = append(indexes, store.SitemapIndex{
			Hash:   store.HashOf(u),
			Domain: domainOf(u),
			URL:    u,
		})
	}
	if err := s.store.CreateSitemapsInBatches(ctx, indexes); err != nil {
		return sitemaperr.Wrap("create sitemaps", err)
	}

	s.log.Info().
		Str("site", siteURL).
		Int("sitemapindexes", len(robots.Sitemaps)).
		Int("leaves", len(leafURLs)).
		Msg("ScanSite complete")
	return nil
}

// ScanSitemapIndex walks up to limit unscanned sitemaps,
// fetching each one, parsing its <urlset>, and persisting the
// resulting links. Sitemaps are marked scanned at the end.
//
// The walk uses an errgroup with cfg.Concurrency workers; the
// first error cancels the rest. ctx cancellation also
// propagates.
func (s *Scanner) ScanSitemapIndex(ctx context.Context, limit int) error {
	if s == nil {
		return errors.New("scanner: nil receiver")
	}
	indexes, err := s.store.GetSitemapIndexToScan(ctx, limit)
	if err != nil {
		return sitemaperr.Wrap("get sitemaps", err)
	}
	if len(indexes) == 0 {
		s.log.Info().Msg("no sitemaps to scan")
		return nil
	}

	concurrency := s.cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	for _, idx := range indexes {
		idx := idx
		g.Go(func() error {
			s.delay.Wait()
			if err := gctx.Err(); err != nil {
				return err
			}
			body, err := GetOK(gctx, s.fetch, idx.URL)
			if err != nil {
				return sitemaperr.Wrap("fetch urlset", err)
			}
			entries, err := sitemapxml.ParseURLSet(bytesReader(body))
			if err != nil {
				return sitemaperr.Wrap("parse urlset", err)
			}
			links := make([]store.Link, 0, len(entries))
			for _, e := range entries {
				links = append(links, store.Link{
					Hash:         store.HashOf(e.URL),
					Domain:       domainOf(e.URL),
					SitemapIndex: idx.Hash,
					URL:          e.URL,
					Lastmod:      e.Lastmod,
					Changefreq:   e.Changefreq,
					Priority:     e.Priority,
				})
			}
			if err := s.store.CreateLinksInBatches(gctx, links); err != nil {
				return sitemaperr.Wrap("create links", err)
			}
			s.log.Debug().
				Str("sitemap", idx.URL).
				Int("links", len(links)).
				Msg("scanned sitemap")
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	if err := s.store.UpdateScannedSitemaps(ctx, indexes); err != nil {
		return sitemaperr.Wrap("mark scanned", err)
	}
	s.log.Info().
		Int("scanned", len(indexes)).
		Msg("ScanSitemapIndex complete")
	return nil
}

// domainOf is a thin wrapper that returns the host portion of
// u, or "" if unparseable. Defined in fetcher.go to avoid
// duplication.

// bytesReader is a tiny helper to avoid pulling in
// bytes.NewReader for one call.
func bytesReader(b []byte) *byteReader {
	return &byteReader{b: b}
}

type byteReader struct {
	b []byte
	i int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, errEOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}

var errEOF = errors.New("EOF")

// ResolveSiteURL is a small helper for callers that want a
// stable scheme://host form before passing to ScanSite.
func ResolveSiteURL(siteURL string) (string, error) {
	parsed, err := url.Parse(siteURL)
	if err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("site url needs scheme and host: %q", siteURL)
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

// DomainOf is re-exported here so external callers don't have
// to import the internal types package.
func DomainOf(u string) string { return types.DomainFrom(u) }
