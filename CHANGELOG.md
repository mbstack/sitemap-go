# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-07-26

### Changed — breaking
- Module path renamed: `github.com/MegaBytee/sitemap-go` →
  `github.com/mbstack/sitemap-go`.
- Public types renamed: `Url` → `URL`, `ProxyUrl` → `ProxyURL`,
  `SafeUrlParser` → `SafeURLParser`. JSON wire format is unchanged.
- License holder updated: MegaBytee.com → mbstack.dev.
- Go version bumped to 1.25.
- `NewScanner` now returns `(*Scanner, error)` (was `*Scanner`,
  nil-on-error). `ScanSite` and `ScanSitemapIndex` now return `error`.
- The library no longer writes to the host filesystem under
  `<exe-dir>/data`. Use `Config.DataDir` explicitly.

### Added
- `Store` interface in the `store` package, with the gorm-backed
  SQLite implementation in `store/sqlite`. Persistence is now
  dependency-inverted.
- `Fetcher` interface in the root `sitemap` package, with a
  `CollyFetcher` default that uses a fresh `*colly.Collector` per
  call (closes the shared-collector accumulation footgun).
- `sitemapxml` package with pure-XML parsers for `<sitemapindex>`
  and `<urlset>`. No I/O; takes an `io.Reader`.
- Sentinel errors `ErrInvalidConfig`, `ErrNoSitemap`,
  `ErrSiteNotFound`, `ErrFetch` re-exported from
  `internal/sitemaperr`, matchable with `errors.Is`.
- `internal/httpx` — small `*http.Client` builder with timeout,
  optional proxy, optional User-Agent injection.
- `internal/ratelimit` — bounded random delay (replaces the
  audit-flagged `setDelayInMs`).
- `internal/testutil` — `NewTestDB`, `NewTestServer`,
  `NewTestLogger`, `SafeBuffer`.
- `cmd/sitemap` CLI with structured logging and proper exit codes.
- `zerolog` for all library logging; `Config.Logger` is required.
- Context-aware `ScanSite` and `ScanSitemapIndex`; cancellation
  propagates through `errgroup`.
- CI: GitHub Actions workflow running `gofmt`, `go vet`,
  `staticcheck`, and `go test -race`.

### Removed
- `panic("stop scanning")` in `ScanSite`.
- `log.Fatal` calls in `storage/sqlite.go`.
- `os.Executable()`-based path discovery.
- The 24 MB stray ELF binary at `examples/examples`.
- Legacy `storage/` package and the `colly-sqlite3-storage`
  dependency.

### Fixed
- Every file in the repo now passes `gofmt -l`.
- `staticcheck -checks=all` is clean (down from 26 issues).
- The `Site.Sitemaps` field is now a real JSON column with proper
  serialization; the legacy code stored it as a hand-rolled string.
- `<urlset>` parsing now also extracts `<lastmod>`, `<changefreq>`,
  and `<priority>` (previously lost).
