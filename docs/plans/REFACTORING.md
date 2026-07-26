# sitemap-go — Refactoring Plan

**From:** `github.com/MegaBytee/sitemap-go` (prototype, `internal`-less, global
log, hard-coded paths, panics)
**To:** `github.com/mbstack/sitemap-go` (library + CLI, `internal/` layout,
zerolog, dependency-inverted storage, full test strategy, v1.0.0)

This document is the **plan to execute**, not the execution. Each phase
ends with a green build, green tests, and a commit boundary so the work
is bisectable and reviewable.

---

## 0. Guiding principles

1. **Outcome is the same.** A `Scanner` is constructed with a config,
   asked to scan a site, fetches `robots.txt`, finds `sitemap.xml`
   indexes, walks them, persists URLs to SQLite, and emits structured
   logs. Nothing about the public capability changes.
2. **Library + CLI, not just CLI.** The public surface lives in
   `pkg/sitemap` (or stays at the root as `package sitemap` — see
   Phase 1). The CLI in `cmd/sitemap` is a thin caller. Anything that
   isn't library-grade is hidden under `internal/`.
3. **Errors return, never panic, never `log.Fatal`.** Every public
   method returns `error`. Sentinel errors live in `errors.go`.
4. **Side effects are explicit.** Paths, DBs, loggers, clock,
   random — all injected through interfaces or fields on `Config`.
5. **Tested as we go.** Each phase adds tests for what it changed.
   No phase ends with `?` packages.
6. **gorm stays. colly stays. log → zerolog.** Per the brief.
7. **Module path:** `github.com/mbstack/sitemap-go`
8. **License:** MIT, copyright `mbstack.dev` (year updated to 2026).

---

## 1. Target layout (post-refactor)

```
sitemap-go/
├── .github/
│   └── workflows/
│       └── ci.yml                  (gofmt, go vet, staticcheck, go test -race)
├── .gitignore
├── CHANGELOG.md                    (1.0.0 entry: rewrite notes)
├── LICENSE                         (MIT, (c) 2026 mbstack.dev)
├── README.md                       (quickstart, config, CLI usage, library usage)
├── go.mod                          (module github.com/mbstack/sitemap-go, go 1.25)
├── go.sum
│
├── doc.go                          (// Package sitemap ...)
│
├── scanner.go                      (Scanner type, NewScanner, ScanSite,
│                                    ScanSitemapIndex, Close)
├── fetcher.go                      (Fetcher interface + CollyFetcher impl)
├── robots.go                       (RobotsTxt parser)
├── errors.go                       (sentinel errors + helpers)
│
├── config/
│   ├── doc.go
│   └── config.go                   (Config struct, DefaultConfig, Validate)
│
├── logger/
│   ├── doc.go
│   └── logger.go                   (zerolog wrapper, New(cfg) *zerolog.Logger)
│
├── store/
│   ├── doc.go
│   ├── store.go                    (Store interface)
│   ├── models.go                   (Site, SitemapIndex, Link gorm models)
│   └── sqlite/
│       ├── doc.go
│       ├── sqlite.go               (Open(path) (*gorm.DB, error))
│       └── store.go                (Store impl, gorm-backed)
│
├── types/
│   ├── doc.go
│   ├── domain.go                   (DomainFrom, URLToPathSlug, SafeURLParser)
│   └── hash.go                     (Hash256, HasHash, GetHashIDs)
│
├── sitemapxml/                     (pure XML parsing, no I/O)
│   ├── doc.go
│   ├── index.go                    (ParseSitemapIndex(io.Reader) ([]string, error))
│   ├── urlset.go                   (ParseURLSet(io.Reader) ([]URL, error))
│   └── xml.go                      (shared internal types)
│
├── internal/
│   ├── httpx/
│   │   ├── doc.go
│   │   ├── client.go               (NewClient(cfg) *http.Client with timeout,
│   │   │                            proxy, UA, retry)
│   │   └── client_test.go
│   ├── ratelimit/
│   │   ├── doc.go
│   │   ├── limiter.go              (token-bucket-ish delay via zerolog)
│   │   └── limiter_test.go
│   └── testutil/
│       ├── doc.go
│       ├── dbsqlite.go             (NewTestDB(t) *gorm.DB helper)
│       ├── http.go                 (NewTestServer(handler) *httptest.Server)
│       └── logs.go                 (CaptureLogs(t) *bytes.Buffer)
│
├── cmd/
│   └── sitemap/
│       ├── main.go                 (CLI: sitemap scan <url> --data <dir>)
│       └── main_test.go
│
└── *_test.go                       (alongside each .go file)
```

Why this layout (one paragraph each):

- **`cmd/sitemap`** is the only `package main`. The rest of the repo
  is a library. Anyone can `import "github.com/mbstack/sitemap-go"` and
  construct a `*Scanner` without `main` getting in the way.
- **`internal/`** holds things that are implementation, not API:
  HTTP client wiring, rate limiter, test helpers. The Go toolchain
  refuses to let external modules import anything under `internal/`,
  which is exactly the contract you want.
- **`sitemapxml/`** is the pure-XML parser. It takes `io.Reader`,
  returns slices of strings (or typed structs), does no I/O. This
  is what makes the parser unit-testable without a network.
- **`store/`** is the gorm-backed persistence, behind a `Store`
  interface. Tests use `:memory:` SQLite via `internal/testutil`.
- **`logger/`** is the zerolog wiring. Anyone who wants a quieter
  log can construct their own `zerolog.Logger` and pass it in
  `Config.Logger`.

---

## 2. Phase-by-phase plan

Each phase = a self-contained change with its own commit and a green
CI. Phases 2–10 are roughly ordered; 0 and 1 are prerequisites.

---

### Phase 0 — Repo prep (15 min)

**Goal:** clean slate, no broken-state surprises.

Tasks:
- [ ] Delete `examples/examples` (the 24 MB stray ELF binary).
- [ ] `gofmt -w .` and `gofumpt -w .` (if installed).
- [ ] Confirm `go build ./...` and `go vet ./...` are clean.
- [ ] Confirm `staticcheck ./...` is clean except for the
  intentionally-broken things this refactor will fix.

Commit: `chore: pre-refactor cleanup (remove stray binary, gofmt)`

---

### Phase 1 — Module rename + license + Go version (30 min)

**Goal:** the new module path is in place before any code moves.

Tasks:
- [ ] Rewrite `go.mod`:
  - `module github.com/mbstack/sitemap-go`
  - `go 1.25` (current latest stable)
  - add `toolchain go1.25.x` if your dev env is on a different patch
- [ ] `go mod tidy` to refresh `go.sum`.
- [ ] Rewrite `LICENSE`:
  - keep MIT body intact
  - copyright line: `Copyright (c) 2026 mbstack.dev`
- [ ] Search & replace the old module path in *every* file:
  - `go.mod`, all `*.go` (imports), `README.md`, `examples/example.go`
  - (example will be deleted in Phase 9 in favour of `cmd/sitemap`,
    so it's fine to leave it alone or delete it now)
- [ ] Add `CHANGELOG.md` with the v1.0.0 entry header — body filled
  as you go.

Verify:
- [ ] `go build ./...` clean
- [ ] `go list -m` shows `github.com/mbstack/sitemap-go`

Commit: `chore: rename module to github.com/mbstack/sitemap-go, license to mbstack.dev`

---

### Phase 2 — Add `zerolog`, remove std `log` (1 hour)

**Goal:** a single, injectable logger that the entire library uses.

Tasks:
- [ ] Add dep: `github.com/rs/zerolog`
- [ ] Create `logger/logger.go`:
  ```go
  // Package logger provides a configured zerolog.Logger for the
  // sitemap-go library and CLI.
  package logger

  import (
      "io"
      "os"
      "time"

      "github.com/rs/zerolog"
  )

  // Options configures the logger.
  type Options struct {
      Level  string // "debug", "info", "warn", "error" — default "info"
      Pretty bool   // human-readable on TTY; default auto
      Out    io.Writer
  }

  // New returns a configured zerolog.Logger.
  func New(opts Options) *zerolog.Logger {
      if opts.Out == nil { opts.Out = os.Stderr }
      lvl, err := zerolog.ParseLevel(opts.Level)
      if err != nil { lvl = zerolog.InfoLevel }
      zerolog.SetGlobalLevel(lvl)
      zerolog.TimeFieldFormat = time.RFC3339

      var lg zerolog.Logger
      if opts.Pretty {
          lg = zerolog.New(zerolog.ConsoleWriter{Out: opts.Out}).
              With().Timestamp().Logger()
      } else {
          lg = zerolog.New(opts.Out).With().Timestamp().Logger()
      }
      return &lg
  }
  ```
- [ ] Sweep every `.go` file and replace `log.Println` / `log.Printf`
  / `fmt.Println` with the new logger. Each call site gets a `.Str`
  field for context (e.g. `lg.Info().Str("url", url).Msg("scanning")`).
- [ ] **Do not** introduce `slog.Default()` anywhere — zerolog is the
  standard. CLI can still tee to stdout if it wants.

Verify:
- [ ] `go build ./...` clean
- [ ] `staticcheck ./...` clean
- [ ] no `log.` references left in non-test code:
  ```powershell
  Select-String -Path . -Pattern '\blog\.' -Recurse |
      Where-Object { $_.Path -notlike '*_test.go' }
  ```
  → should be empty.

Commit: `feat(logger): add zerolog wrapper, replace std log`

---

### Phase 3 — Errors package (30 min)

**Goal:** every public method returns `error`, with sentinel errors
callers can `errors.Is` against.

Tasks:
- [ ] Create `errors.go` at the repo root:
  ```go
  package sitemap

  import "errors"

  var (
      // ErrInvalidConfig is returned when Config.Validate fails.
      ErrInvalidConfig = errors.New("sitemap: invalid config")

      // ErrNoSitemap is returned when robots.txt has no Sitemap entries.
      ErrNoSitemap = errors.New("sitemap: no sitemap entries in robots.txt")

      // ErrSiteNotFound is returned by the store when a site row is missing.
      ErrSiteNotFound = errors.New("sitemap: site not found")

      // ErrFetch is wrapped around any HTTP failure.
      ErrFetch = errors.New("sitemap: fetch failed")
  )
  ```
- [ ] Add a `Wrap(op string, err error) error` helper in `errors.go`
  using `fmt.Errorf("%s: %w", op, err)`.

Commit: `feat(errors): add sentinel errors and wrap helper`

---

### Phase 4 — `config` package (1 hour)

**Goal:** a validated, defaultable, dependency-injectable `Config`.

Tasks:
- [ ] Rewrite `config/config.go`:
  ```go
  // Package config defines the runtime configuration for a Scanner.
  package config

  import (
      "fmt"
      "net/http"
      "os"
      "path/filepath"
      "time"

      "github.com/rs/zerolog"
  )

  // Config is the validated runtime configuration.
  type Config struct {
      // DataDir is the directory for SQLite DB and Colly cache.
      // Required.
      DataDir string

      // Logger receives all library log output. Required.
      // Tests typically use a logger writing to a bytes.Buffer.
      Logger *zerolog.Logger

      // HTTPClient is used for robots.txt + sitemap fetches.
      // If nil, internal/httpx.NewClient(cfg) is used.
      HTTPClient *http.Client

      // Fetcher is the sitemap/robots fetcher. If nil, the
      // Colly-backed default is used.
      Fetcher Fetcher // interface defined in fetcher.go

      // Store is the persistence layer. If nil, the
      // SQLite-backed default is used.
      Store Store // interface defined in store/store.go

      // Concurrency is the worker pool size for ScanSitemapIndex.
      // Default 16, must be > 0.
      Concurrency int

      // MinDelay and MaxDelay bound the random sleep between
      // requests, to be polite. Default 10ms and 500ms.
      MinDelay time.Duration
      MaxDelay time.Duration

      // UserAgent is sent on every request. Default
      // "mbstack-sitemap-go/1.0 (+https://mbstack.dev)".
      UserAgent string
  }

  // DefaultConfig returns a Config with sensible zero-value defaults
  // filled in. DataDir is still required; Logger is still required.
  func DefaultConfig() Config {
      return Config{
          Concurrency: 16,
          MinDelay:    10 * time.Millisecond,
          MaxDelay:    500 * time.Millisecond,
          UserAgent:   "mbstack-sitemap-go/1.0 (+https://mbstack.dev)",
      }
  }

  // Validate returns ErrInvalidConfig (wrapped) if the Config is
  // not usable. Use the helpers in this package to build a
  // validated config from defaults + user input.
  func (c *Config) Validate() error {
      if c.DataDir == "" {
          return fmt.Errorf("%w: DataDir is required", ErrInvalidConfig)
      }
      if c.Logger == nil {
          return fmt.Errorf("%w: Logger is required", ErrInvalidConfig)
      }
      if c.Concurrency <= 0 {
          c.Concurrency = 16
      }
      if c.MinDelay <= 0 { c.MinDelay = 10 * time.Millisecond }
      if c.MaxDelay <= 0 { c.MaxDelay = 500 * time.Millisecond }
      if c.UserAgent == "" {
          c.UserAgent = "mbstack-sitemap-go/1.0"
      }
      if c.MaxDelay < c.MinDelay {
          return fmt.Errorf("%w: MaxDelay < MinDelay", ErrInvalidConfig)
      }
      return nil
  }

  // EnsureDataDir creates c.DataDir (and parents) if it does not
  // exist. Idempotent.
  func (c *Config) EnsureDataDir() error {
      return os.MkdirAll(c.DataDir, 0o755)
  }

  // CollyDir is where Colly's cache + storage live.
  func (c *Config) CollyDir() string {
      return filepath.Join(c.DataDir, "colly")
  }

  // DBPath is the SQLite file.
  func (c *Config) DBPath() string {
      return filepath.Join(c.DataDir, "sitemap.db")
  }
  ```
- [ ] Delete `config/dataDir.go` (replaced by `EnsureDataDir`).
- [ ] Add `config/config_test.go` (table-driven validation).

Commit: `feat(config): validated Config with defaults, no os.Executable coupling`

---

### Phase 5 — `internal/httpx` HTTP client (1 hour)

**Goal:** one place that builds an `*http.Client` with timeout,
optional proxy, and a UA. Reused by the robots fetcher and Colly.

Tasks:
- [ ] Create `internal/httpx/client.go`:
  ```go
  // Package httpx builds the shared *http.Client used by the
  // sitemap-go library for robots.txt and sitemap fetches.
  package httpx

  import (
      "net/http"
      "net/url"
      "time"
  )

  // Options configures New.
  type Options struct {
      Timeout  time.Duration
      ProxyURL string
      UserAgent string
  }

  // New returns a configured *http.Client. Proxy is applied via
  // http.Transport.Proxy if non-empty.
  func New(opts Options) *http.Client {
      if opts.Timeout == 0 { opts.Timeout = 30 * time.Second }
      tr := &http.Transport{
          Proxy: nil,
      }
      if opts.ProxyURL != "" {
          if pu, err := url.Parse(opts.ProxyURL); err == nil {
              tr.Proxy = http.ProxyURL(pu)
          }
      }
      return &http.Client{
          Timeout: opts.Timeout,
          Transport: tr,
      }
  }
  ```
- [ ] `internal/httpx/client_test.go` — table-driven:
  - timeout is honoured
  - ProxyURL is wired (`http.Get` through a test proxy fails open
    but the transport's `Proxy` field is set — assert via field)
  - default timeout is non-zero

Commit: `feat(internal/httpx): shared http.Client builder`

---

### Phase 6 — `internal/ratelimit` polite delay (45 min)

**Goal:** the `setDelayInMs` footgun from the audit becomes a tested
rate limiter with a documented bound.

Tasks:
- [ ] Create `internal/ratelimit/limiter.go`:
  ```go
  // Package ratelimit provides a polite, bounded random delay
  // between HTTP requests, with a single-shot Wait() call.
  package ratelimit

  import (
      "math/rand/v2"
      "time"
  )

  // Limiter sleeps a uniformly-random duration in [Min, Max]
  // each time Wait is called.
  type Limiter struct {
      Min, Max time.Duration
  }

  // Wait sleeps for a random duration in [Min, Max]. If Min == Max
  // the sleep is exact. If Min > Max the values are swapped.
  func (l Limiter) Wait() {
      min, max := l.Min, l.Max
      if min > max { min, max = max, min }
      if min < 0 { min = 0 }
      if max <= min { time.Sleep(min); return }
      delta := max - min
      // math/rand/v2: top exclusive
      time.Sleep(min + time.Duration(rand.Int64N(int64(delta)+1)))
  }
  ```
- [ ] `internal/ratelimit/limiter_test.go` — assert:
  - 1000 calls produce durations in `[Min, Max]`
  - `Min == 0` works
  - `Min == Max` is exact
  - swap when `Min > Max`

Commit: `feat(internal/ratelimit): bounded random delay`

---

### Phase 7 — `internal/testutil` test helpers (45 min)

**Goal:** every test in the repo can spin up an in-memory SQLite, an
`httptest.Server`, and capture log output with three lines.

Tasks:
- [ ] `internal/testutil/dbsqlite.go`:
  ```go
  // Package testutil provides shared helpers for the sitemap-go
  // test suite: in-memory SQLite, httptest servers, and log
  // capture.
  package testutil

  import (
      "testing"

      "github.com/mbstack/sitemap-go/store/sqlite"
      "github.com/mbstack/sitemap-go/store"
      "github.com/rs/zerolog"
      "gorm.io/gorm"
  )

  // NewTestDB returns a fresh :memory: gorm.DB and registers a
  // t.Cleanup that closes it.
  func NewTestDB(t *testing.T) *gorm.DB {
      t.Helper()
      db, err := sqlite.OpenMemory()
      if err != nil { t.Fatalf("open memory sqlite: %v", err) }
      t.Cleanup(func() {
          sqlDB, _ := db.DB()
          if sqlDB != nil { _ = sqlDB.Close() }
      })
      if err := store.Migrate(db); err != nil {
          t.Fatalf("migrate: %v", err)
      }
      return db
  }

  // NewTestLogger returns a zerolog.Logger that writes to a
  // thread-safe bytes.Buffer, returned alongside it.
  func NewTestLogger(t *testing.T) (*zerolog.Logger, *SafeBuffer) {
      t.Helper()
      buf := &SafeBuffer{}
      lg := zerolog.New(buf).Level(zerolog.DebugLevel)
      return &lg, buf
  }
  ```
- [ ] `internal/testutil/http.go` — `NewTestServer(handler) *httptest.Server`
  that registers `t.Cleanup(s.Close)`.
- [ ] `internal/testutil/logs.go` — `SafeBuffer` is a thin
  `bytes.Buffer` + `sync.Mutex` wrapper (zerolog is concurrent-safe
  but the test assertions on the buffer need a stable snapshot).
- [ ] `internal/testutil/doc.go` with a worked example.

Commit: `feat(internal/testutil): in-memory DB, httptest, log capture`

---

### Phase 8 — `store` package + gorm models (2 hours)

**Goal:** persistence is behind a `Store` interface, with a
gorm-backed SQLite default. Models live next to the interface so the
schema is co-located with the contract.

Tasks:
- [ ] `store/store.go`:
  ```go
  // Package store defines the persistence contract for a Scanner
  // and the gorm model types it operates on.
  package store

  import (
      "context"

      "github.com/mbstack/sitemap-go/types"
  )

  // Site, SitemapIndex, and Link mirror the wire types the library
  // produces. They live here (not in types/) so the store can
  // own the gorm tags and migrations.
  type Site struct {
      ID        uint   `gorm:"primaryKey"`
      Domain    string `gorm:"uniqueIndex;not null"`
      URL       string `gorm:"not null"`
      Sitemaps  string `gorm:"type:text"` // JSON-encoded []string
      CreatedAt int64  `gorm:"autoCreateTime"`
      UpdatedAt int64  `gorm:"autoUpdateTime"`
  }
  // ... SitemapIndex, Link analogous.

  // Store is the persistence contract.
  type Store interface {
      SaveSite(ctx context.Context, s *Site) error
      GetSiteByDomain(ctx context.Context, domain string) (*Site, error)

      CreateSitemapsInBatches(ctx context.Context, ss []SitemapIndex) error
      GetSitemapIndexToScan(ctx context.Context, limit int) ([]SitemapIndex, error)
      UpdateScannedSitemaps(ctx context.Context, ss []SitemapIndex) error

      CreateLinksInBatches(ctx context.Context, ls []Link) error
      Close() error
  }

  // Migrate runs AutoMigrate for all store-owned models.
  func Migrate(db *gorm.DB) error {
      return db.AutoMigrate(&Site{}, &SitemapIndex{}, &Link{})
  }
  ```
- [ ] `store/sqlite/sqlite.go`:
  ```go
  // Package sqlite is the default gorm-backed SQLite Store.
  package sqlite

  import (
      "gorm.io/driver/sqlite"
      "gorm.io/gorm"
  )

  // Open opens (or creates) a SQLite DB at path.
  func Open(path string) (*gorm.DB, error) {
      return gorm.Open(sqlite.Open(path), &gorm.Config{})
  }

  // OpenMemory opens an in-memory :memory: SQLite. Used in tests.
  func OpenMemory() (*gorm.DB, error) {
      return gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
  }
  ```
- [ ] `store/sqlite/store.go` — the `*Store` struct that implements
  `store.Store`, with all the methods from the audit's `storage/data.go`
  but rewritten to:
  - take `ctx context.Context`
  - return `error` (not log)
  - use `OnConflict{DoNothing: true}` consistently
  - use `db.WithContext(ctx)`
- [ ] `store/store_test.go`:
  - `TestSaveSite_GetSiteByDomain` — round trip
  - `TestCreateSitemapsInBatches_DeDup` — insert twice, count == 1
  - `TestGetSitemapIndexToScan_OnlyUnscanned` — insert 5, scan 2,
    call `GetSitemapIndexToScan(10)`, expect 3
  - `TestUpdateScannedSitemaps_Batch` — insert 250, update in
    batches of 100, expect all marked scanned
  - `TestCreateLinksInBatches` — basic

Commit: `feat(store): Store interface + gorm SQLite impl, ctx-aware`

---

### Phase 9 — `sitemapxml` pure parser (1.5 hours)

**Goal:** `sitemapindex.xml` and `urlset.xml` are parsed by a
zero-dependency pure-XML function. This is the most testable part
of the codebase and the foundation for the new fetcher.

Tasks:
- [ ] `sitemapxml/index.go`:
  ```go
  // Package sitemapxml parses sitemap index and urlset XML
  // documents into typed Go values, with no I/O.
  package sitemapxml

  import (
      "encoding/xml"
      "fmt"
      "io"
  )

  type sitemapIndex struct {
      XMLName xml.Name `xml:"sitemapindex"`
      Sitemaps []sitemapEntry `xml:"sitemap"`
  }

  type sitemapEntry struct {
      Loc string `xml:"loc"`
  }

  // ParseSitemapIndex returns every <loc> in a <sitemapindex>.
  // Returns an empty slice (not nil) on an empty document.
  func ParseSitemapIndex(r io.Reader) ([]string, error) {
      var idx sitemapIndex
      if err := xml.NewDecoder(r).Decode(&idx); err != nil {
          return nil, fmt.Errorf("decode sitemapindex: %w", err)
      }
      out := make([]string, 0, len(idx.Sitemaps))
      for _, s := range idx.Sitemaps {
          if s.Loc != "" { out = append(out, s.Loc) }
      }
      return out, nil
  }
  ```
- [ ] `sitemapxml/urlset.go` — same shape for `<urlset>/<url>/<loc>`.
  Bonus: also pull `<lastmod>`, `<changefreq>`, `<priority>` into a
  struct so the audit's "no lastmod extraction" gap is closed.
- [ ] `sitemapxml/index_test.go` + `urlset_test.go` — golden-file
  tests with `strings.NewReader` inputs.
  - Empty document
  - Single sitemap
  - Many sitemaps
  - Malformed XML → error
  - Whitespace handling
  - `urlset` includes lastmod/changefreq/priority

Commit: `feat(sitemapxml): pure-XML sitemapindex and urlset parsers`

---

### Phase 10 — Colly fetcher + `Fetcher` interface (2 hours)

**Goal:** the existing Colly `OnXML` callbacks are wrapped behind a
`Fetcher` interface. The default implementation uses Colly; tests
can pass a fake.

Tasks:
- [ ] `fetcher.go`:
  ```go
  // Fetcher fetches a robots.txt or sitemap URL and returns the
  // raw bytes. Implementations: CollyFetcher (default), HTTPFetcher.
  type Fetcher interface {
      Get(ctx context.Context, url string) (body []byte, err error)
  }
  ```
- [ ] `fetcher_colly.go` — wraps `*colly.Collector`:
  - builds a fresh collector per call (closes the F16 footgun)
  - calls `c.Visit(url)` then `c.Wait()`
  - returns the body buffer
  - uses `internal/httpx.New(...)` for transport
  - respects `cfg.MinDelay` / `cfg.MaxDelay` via `internal/ratelimit`
- [ ] `fetcher_test.go` — using `internal/testutil.NewTestServer`:
  - happy path
  - 4xx returns `ErrFetch`
  - 5xx returns `ErrFetch`
  - non-XML body doesn't crash the parser (the parser is in
    `sitemapxml`, not here, so this is just an integration check)
  - context cancellation is honoured

Commit: `feat(fetcher): Fetcher interface with Colly default`

---

### Phase 11 — `robots` package (1 hour)

**Goal:** `GetSitemapURLs(ctx, url)` replaces
`GetSitemapUrlsFromRobotsTxt`. Returns `error`, uses `ctx`, parses
case-insensitive, trims comments, handles BOM.

Tasks:
- [ ] `robots.go`:
  ```go
  // Package robots (file lives at repo root) fetches and parses
  // robots.txt, returning any Sitemap directives.
  //
  // Spec: https://www.rfc-editor.org/rfc/rfc9309.html
  package sitemap

  // RobotsTxt is the result of parsing a robots.txt body.
  type RobotsTxt struct {
      Sitemaps []string
  }

  // GetSitemapURLs returns the Sitemap URLs declared in the
  // robots.txt at the given base URL. Returns ErrNoSitemap if
  // none are present.
  func GetSitemapURLs(ctx context.Context, baseURL string, f Fetcher) (RobotsTxt, error) {
      // 1. resolve baseURL → origin
      // 2. build <origin>/robots.txt
      // 3. f.Get(ctx, robotsURL)
      // 4. parse line-by-line, case-insensitive "sitemap:"
      // 5. skip empty / comment lines (# ...)
      // 6. return ErrNoSitemap if zero
  }
  ```
- [ ] `robots_test.go`:
  - empty robots.txt → ErrNoSitemap
  - one sitemap line
  - multiple sitemap lines
  - mixed case ("Sitemap:" vs "sitemap:")
  - comment lines skipped
  - whitespace tolerance
  - HTTP error → ErrFetch

Commit: `feat(robots): ctx-aware, err-returning robots.txt parser`

---

### Phase 12 — `Scanner` (3 hours)

**Goal:** the central type. The audit's F5/F6/F7/F8 fixes all land
here.

Tasks:
- [ ] `scanner.go`:
  ```go
  // Package sitemap provides Scanner, the central type that
  // fetches a site's robots.txt, extracts its sitemap index(es),
  // walks them, and persists the discovered URLs.
  package sitemap

  import (
      "context"
      "errors"
      "fmt"
      "sync"

      "github.com/mbstack/sitemap-go/config"
      "github.com/mbstack/sitemap-go/store"
      "github.com/mbstack/sitemap-go/sitemapxml"
      "github.com/rs/zerolog"
      "golang.org/x/sync/errgroup"
  )

  // Scanner walks a site's sitemap(s) and persists what it
  // finds. Construct with NewScanner; always Close.
  type Scanner struct {
      cfg    config.Config
      log    *zerolog.Logger
      store  store.Store
      fetch  Fetcher
  }

  // NewScanner constructs a Scanner. Returns ErrInvalidConfig
  // (wrapped) if cfg.Validate fails. If cfg.Store is nil,
  // store/sqlite is opened at cfg.DBPath(). If cfg.Fetcher is
  // nil, a CollyFetcher is built. Always call (*Scanner).Close
  // when done.
  func NewScanner(ctx context.Context, cfg config.Config) (*Scanner, error) {
      if err := cfg.Validate(); err != nil { return nil, err }
      if err := cfg.EnsureDataDir(); err != nil { return nil, err }
      // resolve store, fetch, logger...
  }

  // ScanSite:
  //   1. fetch robots.txt via s.fetch
  //   2. parse Sitemap URLs
  //   3. for each, fetch and ParseSitemapIndex to get leaf URLs
  //   4. SaveSite + CreateSitemapsInBatches
  //   5. return nil or wrapped error
  func (s *Scanner) ScanSite(ctx context.Context, siteURL string) error { ... }

  // ScanSitemapIndex:
  //   1. GetSitemapIndexToScan(limit)
  //   2. errgroup with cfg.Concurrency workers
  //   3. each: fetch + ParseURLSet + CreateLinksInBatches
  //   4. UpdateScannedSitemaps at the end
  func (s *Scanner) ScanSitemapIndex(ctx context.Context, limit int) error { ... }

  // Close releases the store. Safe to call multiple times.
  func (s *Scanner) Close() error { ... }
  ```
- [ ] Rules:
  - **No panics.** Every error path returns.
  - **No `log.Fatal`.** Ever.
  - **No `fmt.Println`.** Always `s.log.Info().Msg(...)`.
  - **Context-aware.** Every public method takes `ctx`.
  - **`errgroup.WithContext`** for the worker pool — the first
    error cancels the rest.
  - **No shared mutable closure state** in Colly callbacks (F16).
- [ ] `scanner_test.go`:
  - happy path: `httptest.Server` serving a robots.txt + a
    sitemapindex + 2 urlsets, verify rows in `Store`
  - 4xx from robots → ErrNoSitemap
  - empty sitemap list → ErrNoSitemap
  - context cancellation mid-scan → returns ctx.Err()
  - one sitemap fails inside errgroup → others still try, first
    error wins
  - `Close` is idempotent

Commit: `feat(scanner): ctx-aware Scanner, no panics, errgroup workers`

---

### Phase 13 — `cmd/sitemap` CLI (1.5 hours)

**Goal:** a runnable CLI that ties everything together and replaces
the old `examples/example.go`.

Tasks:
- [ ] `cmd/sitemap/main.go`:
  ```go
  // Command sitemap is the mbstack/sitemap-go CLI.
  //
  // Usage:
  //   sitemap scan <url> --data <dir> [--concurrency N]
  //                      [--min-delay 10ms] [--max-delay 500ms]
  //                      [--log-level info] [--pretty]
  package main

  func main() {
      // stdlib flag (or github.com/spf13/cobra if you want sub-cmds)
      // 1. parse flags
      // 2. build config
      // 3. logger.New(...)
      // 4. scanner.NewScanner(ctx, cfg)
      // 5. scanner.ScanSite(ctx, url)
      // 6. scanner.ScanSitemapIndex(ctx, 100)
      // 7. scanner.Close()
      // 8. on any error: log + os.Exit(1) — CLI may exit, library may not
  }
  ```
- [ ] `cmd/sitemap/main_test.go`:
  - `--help` exits 0 and prints usage
  - invalid flags exit non-zero
  - end-to-end against an `httptest.Server` (this is the integration
    test for the whole library)

Commit: `feat(cmd/sitemap): CLI replacing examples/example.go`

---

### Phase 14 — Rename: `Url` → `URL`, `UrlParser` → `URLParser`, etc. (1 hour, breaking)

**Goal:** the staticcheck `ST1003` findings are fixed in one shot.

Tasks:
- [ ] `types/domain.go`: `SafeUrlParser` → `SafeURLParser`
- [ ] `config/config.go`: `ProxyUrl` → `ProxyURL` (already done in Phase 4)
- [ ] `types/site.go`, `types/link.go`, `types/sitemapIndex.go`:
  - field rename `Url` → `URL` (Go side)
  - keep `json:"url"` to preserve wire format
- [ ] `robots.go`: `safeUrl` → `safeURL`
- [ ] `store/sqlite/store.go`: `sqlite_db` → `sqliteDB`
- [ ] `store/sqlite/store.go`: `site_sitemaps` → `siteSitemaps`
- [ ] Update every test and every doc comment.
- [ ] Bump to `v1.0.0` in a tagged release:
  - `git tag v1.0.0`
  - `CHANGELOG.md` notes: **BREAKING**: `Url` → `URL` across the
    public API, `SafeUrlParser` → `SafeURLParser`, `ProxyUrl` →
    `ProxyURL`. JSON wire format unchanged.

Verify:
- [ ] `staticcheck -checks=all ./...` clean
- [ ] `gofmt -l .` clean
- [ ] `go test ./... -race` green
- [ ] `go vet ./...` clean

Commit: `refactor!: Url → URL, UrlParser → URLParser (ST1003)`

---

### Phase 15 — CI + lint (1 hour)

**Goal:** every push runs the four checks.

Tasks:
- [ ] `.github/workflows/ci.yml`:
  ```yaml
  name: ci
  on: [push, pull_request]
  jobs:
    test:
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v4
        - uses: actions/setup-go@v5
          with: { go-version: '1.25', cache: true }
        - run: go mod download
        - run: gofmt -l . | tee /tmp/fmt; test ! -s /tmp/fmt
        - run: go vet ./...
        - run: go install honnef.co/go/tools/cmd/staticcheck@latest
        - run: staticcheck ./...
        - run: go test ./... -race -coverprofile=cover.out
        - run: go tool cover -func=cover.out | tail -1
  ```
- [ ] Optional: `golangci-lint` with `gofmt`, `govet`, `staticcheck`,
  `unused`, `errcheck`, `gosimple`, `ineffassign` enabled.

Commit: `ci: GitHub Actions — gofmt, vet, staticcheck, test -race`

---

### Phase 16 — README + godoc examples (1.5 hours)

**Goal:** `pkg.go.dev` and the GitHub README both communicate what
this is, how to use it, and how to run the CLI.

Tasks:
- [ ] Rewrite `README.md`:
  1. Badges: CI, go.dev, license, Go version
  2. One paragraph: what it is
  3. Quickstart (library):
     ```go
     cfg := config.DefaultConfig()
     cfg.DataDir = "./data"
     cfg.Logger = logger.New(logger.Options{Pretty: true})
     s, err := sitemap.NewScanner(context.Background(), cfg)
     if err != nil { return err }
     defer s.Close()
     if err := s.ScanSite(ctx, "https://example.com"); err != nil { return err }
     if err := s.ScanSitemapIndex(ctx, 100); err != nil { return err }
     ```
  4. Quickstart (CLI):
     ```
     go install github.com/mbstack/sitemap-go/cmd/sitemap@latest
     sitemap scan https://example.com --data ./data --pretty
     ```
  5. Configuration table
  6. Architecture: 1-paragraph + ASCII diagram
  7. Limitations (single sitemap pass per call, SQLite only,
     no `lastmod`-based scheduling, etc.)
  8. License section pointing to `LICENSE`
- [ ] Add a `// Example...` godoc test in `scanner_test.go` so
  `pkg.go.dev` renders a runnable example on the `Scanner` symbol.
- [ ] Add `doc.go` to every package.

Commit: `docs: README, godoc examples, package doc comments`

---

### Phase 17 — Final verification (30 min)

**Goal:** the audit's "Quick-win checklist" plus the structural
goals are all green.

- [ ] `gofmt -l .` → empty
- [ ] `go vet ./...` → clean
- [ ] `staticcheck ./...` → clean (zero issues, was 26)
- [ ] `go test ./... -race -coverprofile=cover.out` → all pass
- [ ] `go tool cover -func=cover.out` shows coverage on every
  package
- [ ] `go build ./...` clean
- [ ] `go install ./cmd/sitemap` produces a working binary
- [ ] The binary, run against an `httptest.Server`, ends up with
  rows in the SQLite DB
- [ ] `git tag v1.0.0` and push

---

## 3. Test strategy per utility

The general shape: **every public function has a table-driven test,
every package has at least one integration test, every error path
has a test that triggers it.**

### `types/domain.go`

| Symbol | Test |
| --- | --- |
| `DomainFrom` | `https://x.com/path` → `"x.com"`; `not a url` → `""`; `https://user:pass@host:8080/p?q=1` → `"host"` |
| `URLToPathSlug` | `https://x.com/a/b/` → `"a/b"`; `a/b/c` → `"a/b/c"`; empty input → `""` |
| `SafeURLParser` | valid → `https://x.com`; invalid → error; empty → error |

Style: `func TestDomainFrom(t *testing.T)` with a `[]struct{name, in, want string}` table and `t.Run`.

### `types/hash.go`

| Symbol | Test |
| --- | --- |
| `Hash256` | deterministic — same input → same hash (golden string); different inputs → different hashes |
| `GetHashIDs[T]` | empty slice → empty result; mixed types via interface constraint |

### `store/sqlite/store.go` (gorm-backed Store)

| Method | Test |
| --- | --- |
| `SaveSite` + `GetSiteByDomain` | round trip; second insert with same domain does not duplicate (`OnConflict{DoNothing}`) |
| `CreateSitemapsInBatches` | de-dup via `OnConflict`; respects batch size (insert 250, batch 100) |
| `GetSitemapIndexToScan` | only unscanned rows returned; honours `limit` |
| `UpdateScannedSitemaps` | all rows in the slice marked scanned; rows outside the slice untouched |
| `CreateLinksInBatches` | de-dup by hash; preserves order |
| `Close` | idempotent; safe to call on a closed DB |

Use `internal/testutil.NewTestDB(t)` for every test. Each test gets
its own `:memory:` DB so they can run in parallel with `t.Parallel()`.

### `sitemapxml/`

| Function | Test |
| --- | --- |
| `ParseSitemapIndex` | empty doc; single entry; many entries; nested comments; malformed XML; whitespace |
| `ParseURLSet` | same shape; also verify `<lastmod>`, `<changefreq>`, `<priority>` are extracted |

All tests use `strings.NewReader` — no files, no network. This is
the fastest test set in the repo.

### `internal/httpx/`

| Function | Test |
| --- | --- |
| `New` | timeout is set; default timeout when zero; proxy URL is parsed into the transport's `Proxy` field |

### `internal/ratelimit/`

| Function | Test |
| --- | --- |
| `Limiter.Wait` | 1000 calls produce durations in `[Min, Max]`; `Min == Max` is exact; `Min > Max` swaps; `Min < 0` clamped to 0 |

The timing test should be `t.Parallel()`-friendly: use small delays
(1µs–1ms) so the test takes <100ms total.

### `internal/testutil/`

| Helper | Test |
| --- | --- |
| `NewTestDB` | opens; migrations ran; clean teardown |
| `NewTestLogger` | logger writes to the buffer; buffer is `*SafeBuffer` |
| `NewTestServer` | serves; clean teardown |

### `robots.go`

| Case | Test |
| --- | --- |
| 200 with `Sitemap: ...` | one URL returned |
| 200 with `Sitemap:`, `  Sitemap:`, `sitemap:` (case) | all three parsed |
| 200 with comments (`# ...`) | comments skipped |
| 200 empty | `ErrNoSitemap` |
| 404 | `ErrFetch` (wrapped) |
| context cancelled | `ctx.Err()` |

### `fetcher.go`

| Case | Test |
| --- | --- |
| 200 with XML body | bytes returned |
| 4xx | `ErrFetch` (wrapped) |
| 5xx | `ErrFetch` (wrapped) |
| context cancelled | `ctx.Err()` |
| concurrent calls | each gets its own collector (F16 regression) |

### `scanner.go`

| Case | Test |
| --- | --- |
| Happy path | robots → index → 2 urlsets → rows in store |
| 4xx on robots | returns `ErrNoSitemap` (or wrapped) |
| 4xx on one sitemap index | errgroup returns first error; others may continue or be cancelled |
| context cancellation | `ctx.Err()` returned |
| empty sitemap index | `ErrNoSitemap` |
| Concurrency honoured | configure `Concurrency: 2`, instrument the fetcher, assert no more than 2 in flight |
| Close idempotent | call twice, no panic |

### `cmd/sitemap/main.go`

| Case | Test |
| --- | --- |
| `--help` | exit 0, prints usage |
| missing args | exit non-zero, prints usage |
| end-to-end against `httptest.Server` | data lands in the configured SQLite |

---

## 4. Coverage targets

Suggested floor per package. CI can fail if these drop:

| Package | Target |
| --- | --- |
| `types` | 95% |
| `sitemapxml` | 95% |
| `internal/httpx` | 85% |
| `internal/ratelimit` | 90% |
| `store/sqlite` | 90% |
| `robots.go` | 90% |
| `fetcher.go` | 85% |
| `scanner.go` | 85% |
| `cmd/sitemap` | 70% (CLI surface) |

The pure helpers (`types`, `sitemapxml`) should be near-100%; they
are the foundation.

---

## 5. Commit / release plan

Each phase is a single commit (or a small PR). Tags:

```
v0.1.0  after Phase 6    (logger, config, internal helpers, refactor groundwork)
v0.5.0  after Phase 11   (store + parsers + fetcher + robots, public API mostly stable)
v1.0.0  after Phase 14   (naming, JSON tags, full test suite, README)
v1.0.x  patches          (bug fixes only)
```

`v1.0.0` is the first tag advertised on `pkg.go.dev`. Until then,
external users will see the API churn.

---

## 6. Risk register

| Risk | Mitigation |
| --- | --- |
| Renaming the module breaks anything that already imported the old path | Pre-1.0; only the owner uses it; one rewrite commit |
| Gorm v2 + SQLite locking under concurrent writes | Use `db.WithContext`; rely on GORM's connection pool; add a `TestStore_Concurrent` test |
| Colly `OnXML` callback accumulation (audit F16) | Build a fresh collector per call in `CollyFetcher`; integration test in `fetcher_test.go` |
| `internal/` packages can't be imported externally | Document the contract in `internal/testutil/doc.go`; same for `internal/httpx` and `internal/ratelimit` |
| `errgroup` cancels in-flight HTTP on first error | Document with a test; user can wrap with `retry.Do` outside if they need partial-success |
| `sitemapxml` parsing wrong encoding (UTF-8 BOM) | `xml.NewDecoder` handles BOM; add a test with `﻿` prefix |

---

## 7. Out of scope (for v1.0.0)

These are good ideas, but they would inflate the scope. Land them
in v1.1+:

- Multiple persistence backends (Postgres, MySQL) — the `Store`
  interface is ready; just need additional `store/postgres` etc.
- Recursive sitemap-of-sitemap detection (sitemaps that point to
  other sitemaps beyond `sitemapindex`).
- `lastmod`-based incremental re-scans.
- Output formats other than SQLite (CSV, JSONL, Parquet).
- HTTP retry with backoff (today: a single `http.Client` call,
  failure bubbles up).
- CLI flags for `--dry-run`, `--filter`, `--max-depth`.
- Web UI for the discovered URLs.

---

## 8. Estimated effort

| Phase | Wall time |
| --- | --- |
| 0 — repo prep | 15 min |
| 1 — module + license | 30 min |
| 2 — zerolog | 1 h |
| 3 — errors | 30 min |
| 4 — config | 1 h |
| 5 — internal/httpx | 1 h |
| 6 — internal/ratelimit | 45 min |
| 7 — internal/testutil | 45 min |
| 8 — store + gorm | 2 h |
| 9 — sitemapxml | 1.5 h |
| 10 — fetcher | 2 h |
| 11 — robots | 1 h |
| 12 — Scanner | 3 h |
| 13 — cmd/sitemap | 1.5 h |
| 14 — naming rename | 1 h |
| 15 — CI | 1 h |
| 16 — README + godoc | 1.5 h |
| 17 — final verification | 30 min |
| **Total** | **~21 h** |

Roughly 3 working days, or one long weekend with review.

---

## 9. Definition of done

`v1.0.0` ships when:

- [ ] Every phase above is committed and pushed.
- [ ] `gofmt -l .` is empty.
- [ ] `go vet ./...` is clean.
- [ ] `staticcheck ./...` is clean.
- [ ] `go test ./... -race` is green with the coverage targets met.
- [ ] `go install ./cmd/sitemap` works and the binary end-to-end
      walks an `httptest.Server`-served site and writes rows to a
      real SQLite file.
- [ ] `README.md` and `pkg.go.dev` (after the tag) make the
      library usable in 30 seconds.
- [ ] `CHANGELOG.md` has the v1.0.0 entry.
- [ ] `LICENSE` is `mbstack.dev`, year 2026.
- [ ] `go.mod` is `github.com/mbstack/sitemap-go`, `go 1.25`.

When all boxes are checked, tag `v1.0.0` and announce.
