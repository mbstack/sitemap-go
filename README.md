# sitemap-go

A small Go library and CLI that fetches a site's `robots.txt`, walks the
declared `sitemap.xml` indexes, and persists the discovered URLs to a local
SQLite database. Built on [gocolly/colly](https://github.com/gocolly/colly)
for HTTP, [gorm](https://gorm.io) for storage, and
[zerolog](https://github.com/rs/zerolog) for structured logging.

```
go get github.com/mbstack/sitemap-go
```

## Library quickstart

```go
package main

import (
    "context"
    "log"

    "github.com/mbstack/sitemap-go"
    "github.com/mbstack/sitemap-go/config"
    "github.com/mbstack/sitemap-go/logger"
)

func main() {
    cfg := config.DefaultConfig()
    cfg.DataDir = "./data" // required
    cfg.Logger = logger.New(logger.Options{Level: "info", Pretty: true})

    s, err := sitemap.NewScanner(context.Background(), cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer s.Close()

    if err := s.ScanSite(context.Background(), "https://example.com"); err != nil {
        log.Fatal(err)
    }
    if err := s.ScanSitemapIndex(context.Background(), 100); err != nil {
        log.Fatal(err)
    }
}
```

The Scanner will:

1. Fetch `https://example.com/robots.txt`.
2. Parse every `Sitemap:` line (case-insensitive, comments skipped).
3. Fetch each sitemapindex and parse its `<sitemap><loc>` entries; persist
   them to the `sites` and `sitemap_indices` tables.
4. On `ScanSitemapIndex`, fetch each leaf sitemap in parallel (configurable
   concurrency) and persist `<urlset>/<url>/<loc>` entries to the `links`
   table, including `lastmod`, `changefreq`, and `priority` when present.

## CLI

```
go install github.com/mbstack/sitemap-go/cmd/sitemap@latest

sitemap scan https://example.com --data ./data --pretty
```

Flags:

| Flag | Default | Description |
| --- | --- | --- |
| `--data` | `./data` | directory for the SQLite database and Colly cache |
| `--concurrency` | `16` | worker pool size for `ScanSitemapIndex` |
| `--min-delay` | `10ms` | minimum random sleep between requests |
| `--max-delay` | `500ms` | maximum random sleep between requests |
| `--log-level` | `info` | trace, debug, info, warn, error |
| `--pretty` | `false` | human-readable log output (auto-detected on TTY) |
| `--limit` | `100` | max sitemaps to scan per call |

Exit codes: `0` success, `1` invalid flags / config, `2` fetch / parse /
store error.

## Configuration

`config.Config` is the only input the library cares about. The two required
fields are `DataDir` (a directory the library will create) and `Logger`
(any `*zerolog.Logger`). All other fields have sensible defaults via
`config.DefaultConfig()`.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `DataDir` | `string` | — (required) | root directory for SQLite + Colly cache |
| `Logger` | `*zerolog.Logger` | — (required) | structured logger; all library output goes here |
| `HTTPClient` | `*http.Client` | `nil` | custom HTTP client; nil → use `internal/httpx.New` |
| `Concurrency` | `int` | `16` | worker pool size for `ScanSitemapIndex` |
| `MinDelay` | `time.Duration` | `10ms` | minimum sleep between requests |
| `MaxDelay` | `time.Duration` | `500ms` | maximum sleep between requests |
| `UserAgent` | `string` | `mbstack-sitemap-go/1.0 (+https://mbstack.dev)` | UA sent on every request |
| `ProxyURL` | `string` | `""` | optional HTTP proxy URL |

`Config.Validate` fills zero values from `DefaultConfig` and rejects bad
input with an error wrapping `sitemap.ErrInvalidConfig`.

## Error handling

The library never panics on recoverable errors. Every public method returns
`error`. Sentinel errors can be matched with `errors.Is`:

```go
if errors.Is(err, sitemap.ErrNoSitemap) { /* robots.txt had no Sitemap */ }
if errors.Is(err, sitemap.ErrFetch)    { /* network/HTTP failure */ }
if errors.Is(err, sitemap.ErrInvalidConfig) { /* fix Config and retry */ }
if errors.Is(err, sitemap.ErrSiteNotFound)  { /* Store lookup miss */ }
```

## Architecture

```
cmd/sitemap             CLI thin caller
sitemap (root)          Scanner, NewScanner, ScanSite, ScanSitemapIndex,
                        Fetcher interface, GetSitemapURLs, errors
config                  Config struct, DefaultConfig, Validate
logger                  zerolog wrapper (New)
store                   Store interface, gorm models (Site, SitemapIndex, Link)
store/sqlite            default gorm-backed SQLite implementation
sitemapxml              pure XML parsers (no I/O)
internal/httpx          *http.Client builder
internal/ratelimit      bounded random delay
internal/sitemaperr     sentinel errors
internal/testutil       in-memory DB, httptest, log capture
```

The `internal/` packages are blocked from external imports by the Go
toolchain; they are implementation, not API.

## Limitations (v1.0.0)

- One pass per call; no incremental / `lastmod`-based scheduling.
- Sitemap-of-sitemap recursion is limited to one level (the index).
- SQLite only at the persistence layer (the `Store` interface allows
  other backends but none ship in this version).
- No HTTP retry; a single failure bubbles up via `errgroup`.
- `lastmod`, `changefreq`, and `priority` are extracted from `<urlset>`
  but are not used for filtering or scheduling.

## License

MIT — Copyright (c) 2026 mbstack.dev
