# sitemap-go — Library Audit

**Repo:** `github.com/MegaBytee/sitemap-go`
**Date:** 2026-07-26
**Scope:** Code quality, structure, Go best practices, and a refactoring plan to
make this a production-grade reusable Go library.

---

## 0. How the audit was done

Reproducible commands run from the repo root on Go 1.25.4 (Windows):

| Tool | Command | Result |
| --- | --- | --- |
| `go build` | `go build ./...` | ✅ clean |
| `go vet` | `go vet ./...` | ✅ clean |
| `go test` | `go test ./...` | ❌ no tests in any package |
| `gofmt` | `gofmt -l .` | ❌ every Go file in the repo fails `gofmt` (15/15) |
| `staticcheck -checks=all` | `staticcheck -checks=all ./...` | ❌ 26 issues across 12 files (ST1000, ST1003, ST1020) |

Findings below are anchored to that evidence. Static-analysis output is quoted
inline where useful.

---

## 1. Executive summary

`sitemap-go` is a thin crawler on top of [`gocolly/colly`](https://github.com/gocolly/colly)
that reads `robots.txt`, fetches `sitemap.xml` / `sitemapindex` files, and
persists what it finds to a local SQLite (via GORM). The current shape is closer
to a **prototype / single-user script** than a reusable Go library:

- Builds and vets cleanly, but **does not look like a library**: API surface is
  small, error handling is panicky, logging is global, side effects (filesystem,
  DB, network) happen on `init`-style paths, and there are **zero tests**.
- Layout mixes concerns: a top-level `package sitemap` for orchestration lives
  next to its `config`, `storage`, and `types` sub-packages, but `types` and
  `storage` quietly import the same `config` package for filesystem paths —
  the library cannot run without writing to disk.
- A 24 MB ELF binary (`examples/examples`, ~24,063,928 bytes, magic
  `7F 45 4C 46`) is committed at the repo root. It is not built, not
  referenced, and is not Windows-runnable.
- Style: every Go file in the repo fails `gofmt`; 26 `staticcheck` issues
  (missing package comments, wrong naming convention for `URL`, `URLParser`,
  `ProxyURL`, `sqliteDB`, `safeURL`, `siteSitemaps`; wrong comment style on
  exported methods).

The refactor is **mostly about discipline, not rewriting** — small structural
moves, error semantics, isolation of side effects, and a real test suite will
move this from "script with a module path" to a library you can import.

---

## 2. Repository layout (as found)

```
sitemap-go/
├── .gitignore
├── LICENSE                       (MIT, 2025 MegaBytee.com)
├── README.md                     (1 paragraph, no godoc, no usage example)
├── go.mod                        (module github.com/MegaBytee/sitemap-go, Go 1.24.4)
├── go.sum
├── colly.go                      (package sitemap, private colly setup)
├── robots-txt.go                 (package sitemap, HTTP robots.txt parser)
├── sitemap.go                    (package sitemap, Scanner type)
├── config/
│   ├── config.go                 (Config struct: ProxyUrl, WithProxy, WithCache)
│   └── dataDir.go                (get-or-create data dir under os.Executable())
├── storage/
│   ├── data.go                   (CRUD wrappers around *gorm.DB)
│   ├── sqlite.go                 (NewSqlite + updateOneFieldInBatches)
│   ├── storage.go                (Storage struct, New, Close, Config)
│   └── tables.go                 (GORM AutoMigrate list)
├── types/
│   ├── domain.go                 (DomainFrom, URLToPathSlug, SafeUrlParser)
│   ├── hash.go                   (Hash256, HasHash interface, GetHashIDs[T])
│   ├── link.go                   (Link model + constructors)
│   ├── site.go                   (Site model + JSON Sitemaps field)
│   └── sitemapIndex.go           (SitemapIndex model + constructor)
└── examples/
    ├── example.go                (package main: one-shot demo)
    └── examples                  (24 MB committed ELF binary — stray artifact)
```

---

## 3. What the library does today

End-to-end happy path (see `examples/example.go`):

```go
cfg := config.Config{WithProxy: false, WithCache: true}
scanner := sitemap.NewScanner(&cfg)              // returns nil on any error
scanner.ScanSite("https://megabytee.com/")       // 1) GET robots.txt
                                                  //    2) extract sitemapindex URLs
                                                  //    3) SaveSite + CreateSitemapsInBatches
scanner.ScanSitemapIndex(100)                    // 4) fan-out 100 sitemap files
                                                  //    at concurrency 100, persist Link[]
scanner.Close()                                   // 5) close DB
```

What `NewScanner` actually does internally (paraphrased from
`sitemap.go:20` + `colly.go:17`):

1. Build a Colly collector (`colly.NewCollector()`).
2. Get-or-create `<exe-dir>/data/`, then `<exe-dir>/data/colly/`.
3. Set Colly storage to SQLite at `<exe-dir>/data/colly/sitemap.db` via
   `velebak/colly-sqlite3-storage`.
4. Optionally set cache dir, optionally set proxy.
5. Build a GORM-backed `*storage.Storage` at `<exe-dir>/data/storage.db`,
   call `AutoMigrate` on it. Return `nil` on any failure.

---

## 4. Findings — Go best-practice violations

Each finding cites the file and the actual code. Severity is my judgement for
"is this a blocker for calling this a Go library".

### 4.1 Library hygiene

#### F1. Committed 24 MB binary — `examples/examples`  ·  **CRITICAL**

```
$ ls -l examples/examples        # 24,063,928 bytes
$ head -c4 examples/examples     # 7F 45 4C 46   (ELF magic)
```

This is a Linux ELF binary accidentally committed inside the `examples/`
directory. It is not referenced, not built by `go build`, and breaks the rule
"don't ship build artifacts". It bloats the repo and surfaces the wrong thing
on Windows.

**Action:** delete it, add a strict `.gitignore` rule for stray executables,
consider `git filter-repo` to scrub it from history if you care about the size.

#### F2. Zero tests  ·  **CRITICAL**

```
?   github.com/MegaBytee/sitemap-go            [no test files]
?   github.com/MegaBytee/sitemap-go/config     [no test files]
?   github.com/MegaBytee/sitemap-go/examples   [no test files]
?   github.com/MegaBytee/sitemap-go/storage    [no test files]
?   github.com/MegaBytee/sitemap-go/types      [no test files]
```

A library with no tests is a library that can break on the first refactor.
The pure helpers in `types/` (`DomainFrom`, `Hash256`, `GetHashIDs`,
`SafeUrlParser`, `Link.HashID`, etc.) are trivially testable today and would
catch naming changes and the `URL` → `Url` rename below.

#### F3. Every file fails `gofmt`  ·  **HIGH**

```
$ gofmt -l .
colly.go
config\config.go
config\dataDir.go
examples\example.go
robots-txt.go
sitemap.go
storage\data.go
storage\sqlite.go
storage\storage.go
storage\tables.go
types\domain.go
types\hash.go
types\link.go
types\site.go
types\sitemapIndex.go
```

Mostly tab-vs-space and comment alignment. Trivial to fix; `gofmt -w .`
followed by `gofumpt` (optional) is the right move, and a CI step that
runs `gofmt -l` on PRs will prevent regressions.

#### F4. Missing package comments on every package  ·  **MEDIUM**

`staticcheck -checks=all` flags 11 occurrences of `ST1000` (one per Go file).
Per `go doc` convention, every package must have a `// Package foo ...`
comment in `doc.go`. Today `godoc` and `pkg.go.dev` will render empty
package pages.

### 4.2 Naming (`staticcheck ST1003`)

Per the Go community style guide ([Effective Go], [Go Code Review Comments]):

| File | Symbol | Issue | Fix |
| --- | --- | --- | --- |
| `config/config.go:5` | `Config.ProxyUrl` | initialism should be `ProxyURL` | `ProxyURL string` |
| `robots-txt.go:21` | local `safeUrl` | should be `safeURL` | `safeURL` |
| `storage/sqlite.go:18` | local `sqlite_db` | underscore in Go name | `sqliteDB` |
| `types/domain.go:32` | `func SafeUrlParser` | initialism | `SafeURLParser` |
| `types/site.go:7` | `Site.Url` | initialism | `Site.URL` |
| `types/link.go:7` | `Link.Url` | initialism | `Link.URL` |
| `types/sitemapIndex.go:6` | `SitemapIndex.Url` | initialism | `SitemapIndex.URL` |
| `types/sitemapIndex.go:23` | local `site_sitemaps` | underscore | `siteSitemaps` |

This is a **breaking API change** for any downstream user (the JSON tags also
change shape unless you keep `json:"url"`). Do it in a single 0.x → 1.0
release with a clear changelog.

### 4.3 Doc comments (`staticcheck ST1020`)

Exported methods / functions whose doc comments do not start with the symbol
name (Go convention — your `godoc` is the README for this kind of library):

- `sitemap.go:39` — `// scan site sitemaps` should be `// ScanSite ...`
- `sitemap.go:73` — `// scan sitemapindex and extract links`
- `sitemap.go:110` — `// extract more sitemapIndex from a sitemapIndex if has any`
- `types/hash.go:22` — `// Helper function to extract IDs from links`
  should start with `// GetHashIDs ...`

### 4.4 Errors and control flow — *the* library-shape problem

#### F5. `NewScanner` returns `nil` on error with no error  ·  **CRITICAL**

```go
// sitemap.go:20
func NewScanner(cfg *config.Config) *Scanner {
    c, err := newCollyScrapper(cfg)
    if err != nil {
        return nil
    }
    data := storage.New().Config()
    if data == nil {
        return nil
    }
    return &Scanner{c: c, data: data}
}
```

The caller has **no way to know why** construction failed. `example.go` then
does `panic("stop here")` if the scanner is nil, which is the only way a
caller can react. The idiomatic Go library signature is:

```go
func NewScanner(cfg *config.Config) (*Scanner, error)
```

This forces every error path to be explicit and is the single highest-value
change in this audit.

#### F6. `panic` inside a library  ·  **CRITICAL**

```go
// sitemap.go:46
robotsTxt, err := GetSitemapUrlsFromRobotsTxt(url)
if err != nil {
    panic("stop scanning")
}
```

A library **must not** `panic` on a recoverable condition. Fetching
`robots.txt` can fail for many legitimate reasons (404, 5xx, redirect loop,
TLS, rate-limit). The caller — not the library — should decide whether to
abort. Replace with `return err` (after F5).

#### F7. Swallowed errors via `log.Printf`  ·  **HIGH**

```go
// sitemap.go:59
err = s.data.SaveSite(newSite)
if err != nil {
    log.Printf("something went wrong:err %v", err)
}
```

Library code should **return** errors, not `log` them. The current pattern
silently loses a write failure; the caller believes the scan succeeded. A
collector / scraper is exactly the kind of code where a transient DB write
failing should bubble up.

#### F8. `log.Fatal` inside package code  ·  **HIGH**

```go
// storage/sqlite.go:15, 22
log.Fatalf("Failed to get project main directory: %v", err)
return nil
log.Fatalf("failed to connect database")
return nil
```

`log.Fatal` calls `os.Exit(1)`. A library that calls `os.Exit` will take down
the caller's process. It is also unreachable (`Fatal` does not return), so the
following `return nil` is dead code. Replace with returning the wrapped error
to the caller.

#### F9. Global mutable logger / no observability hook  ·  **MEDIUM**

The library uses `log.Println` and `fmt.Println` in 8+ places
(`sitemap.go:54,70,107,117`; `colly.go:59,64,67,74`;
`robots-txt.go:41,65,117`). A library that writes to the **standard log
package** makes itself incompatible with any structured-logging host
(slog, zerolog, zap). Today there is no way for the consumer to redirect
these messages.

Idiomatic fix: accept an optional `*slog.Logger` (or a `func(string)` hook)
in `Config`, default to `slog.Default()`. This single change is the biggest
DX win after F5.

### 4.5 Side effects and coupling — the "where does data go?" problem

#### F10. Library writes under `os.Executable()` path  ·  **CRITICAL**

```go
// config/dataDir.go:11
execPath, err := os.Executable()
...
execDir := filepath.Dir(execPath)
newDirPath := filepath.Join(execDir, "data")
```

This means the library will create a `data/` directory **next to the binary
that imports it**. For a library:

- This silently mutates the host filesystem.
- It breaks when the host binary is read-only or in `$GOPATH/bin`.
- It breaks the moment someone embeds the library in a Windows GUI app
  (no write access to `C:\Program Files\...`).
- It is impossible to use in tests without `os.Chdir` hacks.

The SQLite DBs, Colly storage, and HTTP cache dirs all flow from this one
function, so the whole library is implicitly tied to a single hard-coded
path scheme. The fix is to make the data dir a required `Config` field, or
to fall back to `os.UserCacheDir()`.

#### F11. `storage.New()` is a hard-coded SQLite, no DI  ·  **HIGH**

```go
// storage/storage.go:15
func New() *Storage {
    return &Storage{
        Data: NewSqlite("storage.db"),
    }
}
```

The `storage` package can only talk to SQLite. A library that hard-codes its
persistence engine is not a library, it's an app with an import path. Define
a `Repository` interface (`SaveSite`, `CreateSitemapsInBatches`,
`GetSitemapIndexToScan`, `UpdateScannedSitemaps`, `CreateLinksInBatches`,
`Close`) and let `Scanner` depend on the interface. SQLite is the default
implementation; users bring their own DB.

#### F12. `getOrCreateDir` is reinvented  ·  **LOW**

```go
// config/dataDir.go:23
if _, err := os.Stat(newDirPath); os.IsNotExist(err) {
    err := os.Mkdir(newDirPath, os.ModePerm)
```

Idiomatically `os.MkdirAll` does this in one line and is safer
(multi-level paths, race-tolerant). Use `os.MkdirAll(path, 0o755)`.

#### F13. The custom `updateOneFieldInBatches` partially duplicates GORM  ·  **LOW**

```go
// storage/sqlite.go:30
for i := 0; i < len(ids); i += batchSize {
    end := i + batchSize
    if end > len(ids) { end = len(ids) }
    if err := tx.Model(model).Where(whereIn, ids[i:end]).Update(key, value).Error; ...
```

Two issues:

1. `db.Model(...).Where(...).Update(...)` already accepts slices; the manual
   batch loop is unnecessary.
2. The transaction `defer recover()` does not `Rollback` on success
   when the recover branch is taken — only inside the recover. Standard
   pattern is `defer func(){ if err != nil { tx.Rollback() } }()` after each
   statement that can fail.

#### F14. Path-style and `<exe>/data` collide with the import path  ·  **MEDIUM**

`storage.New()` writes to `<exe>/data/storage.db` and `colly.go` writes to
`<exe>/data/colly/sitemap.db`. These two are in the same directory but use
different filenames in different code paths. Move to a single
`DataDir` config, or accept two distinct fields (`CollyDir`, `DBPath`).

### 4.6 Concurrency and resource lifecycle

#### F15. `ScanSitemapIndex` ignores per-goroutine errors  ·  **HIGH**

```go
// sitemap.go:86
go func(index types.SitemapIndex) {
    defer wg.Done()
    sem <- struct{}{}
    urls := s.GetLinksFromSitemapIndex(index.Url)
    if len(urls) > 0 {
        links := types.NewLinksFromSitemapIndex(urls)
        s.data.CreateLinksInBatches(links)   // error discarded
    }
    <-sem
}(index)
```

The persist call returns an error that is dropped. Combined with F7 there
is **no path** by which a write failure reaches the caller.

The semaphore of 100 is also fixed and undocumented; an `errgroup` with
`SetLimit(n)` is the modern equivalent and integrates `error` propagation.

#### F16. The collector is shared across all sitemap requests  ·  **MEDIUM**

```go
// sitemap.go:113
s.c.OnXML("//sitemapindex/sitemap", func(e *colly.XMLElement) { ... })
err := s.c.Visit(url)
```

The Colly collector is a single instance with a single set of
`OnXML`/`OnHTML` callbacks. Because the library appends handlers
unconditionally before each `Visit`, **callbacks accumulate** across
iterations of the loop. After visiting 50 sitemap files, every subsequent
visit fires 50 copies of the handler and appends duplicates. Even
if a handler is replaced (`OnXML` actually replaces, not appends — verify
against colly v2.2.0), the design is fragile: a single shared collector
plus per-call state (`urls []string` captured in a closure) is racy if the
collector ever reuses state.

**Fix:** build a fresh `colly.Collector` per request (or per worker), or
refactor the XML walk to use a streaming parser (`encoding/xml`) — the
Colly indirection adds little here.

#### F17. `sync.RWMutex` on `storage.Storage` is correct but overkill  ·  **LOW**

GORM's `*gorm.DB` is already safe for concurrent use; serialising every
read with `RLock` is fine for a single-process scanner but unnecessary.
Acceptable as-is; flag only if benchmarks show contention.

#### F18. Random-delay `setDelayInMs` blocks the whole `OnRequest`  ·  **MEDIUM**

```go
// colly.go:72
func setDelayInMs(x, y int) {
    delay := rand.Intn(y) + x
    time.Sleep(time.Duration(delay) * time.Millisecond)
}
```

Called from inside `OnRequest`, this sleep happens on the Colly worker
goroutine. With concurrency 100 and a 0–500 ms random delay, request
throughput is throttled. There is no upper bound guarantee: `rand.Intn(500)+10`
yields `[10, 509]` ms. Use `math/rand/v2`, document the bound, and
expose it via `Config` (`MinDelay`, `MaxDelay`). Also `rand.Intn` does
**not** need a `*rand.Rand` seed in modern Go — but `math/rand` was
deprecated as of Go 1.20 in favor of `math/rand/v2`, and you should use
the v2 package for new code.

### 4.7 Public API and docs

#### F19. README is one paragraph, README does not match code  ·  **MEDIUM**

```md
# sitemap-go
A small Go library that uses Colly to fetch a site's sitemap.xml,
parse its URLs, and crawl each linked page to extract additional
sitemap links. Lightweight, concurrent, and easy to integrate into
other projects.
```

This is the entire `README.md`. There is no:

- Go-get / install snippet,
- Quick example (the only example lives in `examples/example.go` and is
  not the canonical godoc example),
- `pkg.go.dev` badge,
- License badge (LICENSE is MIT and present, just not advertised),
- "What this **doesn't** do" / "limitations" section,
- Badges: `go test ./...` status, `gofmt -l .` status, staticcheck.

A reusable Go library today is judged in 30 seconds by its README +
`pkg.go.dev` page. Right now both are barren.

#### F20. `examples/example.go` panics on nil  ·  **MEDIUM**

```go
// examples/example.go:16
scanner := sitemap.NewScanner(&cfg)
if scanner == nil {
    panic("stop here")
}
```

Once F5 is applied this becomes:

```go
scanner, err := sitemap.NewScanner(&cfg)
if err != nil { log.Fatal(err) }
```

…which is the canonical pattern. Update the example in the same PR.

#### F21. `examples/examples` (the 24 MB ELF) is misclassified as an example  ·  **LOW**

Already covered in F1; flagging again because the example directory should
only contain runnable, small, human-readable `.go` files.

#### F22. `Config` has no validation, no defaults, no zero-value safety  ·  **LOW**

```go
type Config struct {
    ProxyUrl  string
    WithProxy bool
    WithCache bool
}
```

Today: `ProxyURL = ""` and `WithProxy = true` ⇒ `c.SetProxy("")` is called
and returns an error, swallowed by `NewScanner` returning `nil`. The library
should:

- Validate (`WithProxy` requires non-empty `ProxyURL`).
- Provide `DefaultConfig()` for sane defaults.
- Document the zero value.

### 4.8 Module / dependency hygiene

#### F23. `go.mod` is Go 1.24.4 but no `toolchain` directive  ·  **LOW**

`go.mod` says `go 1.24.4`. The build runs on 1.25.4 (your machine). Either
bump to `go 1.25` or set `toolchain go1.24.4` explicitly. Otherwise users on
older Go versions will be pulled onto 1.25 by the toolchain selection logic.

#### F24. Three heavy direct dependencies for a small surface  ·  **LOW**

- `github.com/gocolly/colly/v2` — used for HTTP + a few callbacks. Could
  be replaced by `net/http` + `encoding/xml` for the sitemap use case.
- `github.com/velebak/colly-sqlite3-storage` — unmaintained (last commit
  2024-04, see import in `go.sum`).
- `gorm.io/driver/sqlite` + `gorm.io/gorm` — the entire storage layer.

If the goal is a **library** that other projects will import, those three
pulls are a real cost. Consider:

- Colly: keep if you value the cache/proxy/UA infrastructure; otherwise
  drop to `net/http` + `encoding/xml` (one file, ~100 lines).
- `colly-sqlite3-storage`: replace with Colly's built-in `colly.Cache` or
  a thin `bolt`-style store.
- GORM: replace with `database/sql` + a tiny migration helper, or make
  the storage layer a `Repository` interface (see F11) so users can plug
  in their own driver.

If the goal is **just to ship a tool**, none of this matters; keep the deps.

---

## 5. Refactoring plan (proposed)

Ordered by value-per-effort. Each step ends in a green build and a
green test run, so the work is bisectable.

### Phase 0 — Hygiene (1–2 hours)

1. `gofmt -w .` and `gofumpt -w .` (if installed).
2. Add a CI script (`.github/workflows/ci.yml`) that runs
   `gofmt -l`, `go vet ./...`, `staticcheck ./...`, `go test ./... -race`.
3. Delete `examples/examples` (the 24 MB ELF).
4. Add `*.exe`, `*.test`, `data/`, `*.db` to `.gitignore` (data dir and DB
   files are already implicit but make it explicit).
5. Add a `// Package ...` comment to every package, in a new `doc.go` per
   package, so `pkg.go.dev` and `go doc` render correctly.

### Phase 1 — Test the testable (half a day)

Even before refactoring, the helpers are pure functions and trivially
testable. Land a `*_test.go` for each:

- `types/domain_test.go` — `DomainFrom`, `URLToPathSlug`, `SafeURLParser`
  (input/output table tests with `httptest`).
- `types/hash_test.go` — `Hash256` (deterministic), `GetHashIDs[T]` (golden
  data).
- `types/link_test.go` — `NewLink`, `NewLinksFromSitemapIndex`.
- `types/sitemapIndex_test.go` — same shape.
- `types/site_test.go` — `SetSitemaps` round-trip via `ParseSitemaps`.
- `storage/sqlite_test.go` — open `:memory:` SQLite, exercise the
  `Repository` interface, `t.Cleanup` close.

Use `github.com/stretchr/testify/assert` if you want; otherwise stdlib
testing is fine. **No mocking framework needed** — these are pure
functions and an in-memory SQLite.

### Phase 2 — Error semantics (1 day)

1. Change the public API to:
   ```go
   func NewScanner(cfg *config.Config) (*Scanner, error)
   ```
2. Change every public method (`ScanSite`, `ScanSitemapIndex`) to return
   `error`.
3. Replace `log.Printf` in `sitemap.go`, `robots-txt.go` with
   `return fmt.Errorf("...: %w", err)`.
4. Replace `log.Fatal` in `storage/sqlite.go` with `return nil, err`
   (after F11), and have `New` return `error`.
5. Replace `panic("stop scanning")` in `sitemap.go:46` with `return err`.
6. Wrap with `fmt.Errorf` and `%w` so callers can `errors.Is` /
   `errors.As` to your sentinel errors. Introduce a small
   `errors.go` per package with `ErrNotFound`, `ErrInvalidConfig`, etc.

This phase is the highest-leverage change for "library-ness".

### Phase 3 — Configuration and side effects (1 day)

1. Add to `Config`:
   ```go
   type Config struct {
       // Required: directory for SQLite + Colly cache.
       DataDir string

       // Optional: HTTP client + proxy.
       HTTPClient      *http.Client
       ProxyURL        string
       WithProxy       bool
       WithCache       bool
       MinDelay        time.Duration // default 10ms
       MaxDelay        time.Duration // default 500ms
       Concurrency     int           // default 16
       Logger          *slog.Logger  // default slog.Default()
   }
   ```
2. Add `DefaultConfig() Config` that fills the zero values sensibly.
3. Validate in `NewScanner` (return `error` if `DataDir == ""`,
   if `WithProxy && ProxyURL == ""`, etc.).
4. `os.MkdirAll(cfg.DataDir, 0o755)` instead of `config.GetDataDir()`.
5. Stop calling `os.Executable()`. The library never needs to know where
   the host binary lives.

### Phase 4 — Decouple storage (1 day)

1. Define a `Repository` interface in a new `internal/repo` (or
   `sitemap/repo`) package:
   ```go
   type Repository interface {
       SaveSite(ctx context.Context, s *types.Site) error
       CreateSitemapsInBatches(ctx context.Context, ss []types.SitemapIndex) error
       GetSitemapIndexToScan(ctx context.Context, limit int) ([]types.SitemapIndex, error)
       UpdateScannedSitemaps(ctx context.Context, ss []types.SitemapIndex) error
       CreateLinksInBatches(ctx context.Context, ls []types.Link) error
       Close() error
   }
   ```
2. Add `context.Context` to every method so callers can cancel
   long-running scans.
3. Keep `storage/sqlite.go` as the default implementation.
4. `Scanner` takes a `Repository`, not a `*storage.Storage`.

### Phase 5 — Drop the global `log`, use `slog` (half a day)

1. Replace every `log.Println`, `log.Printf`, `fmt.Println` in library
   code with `cfg.Logger.Info(...)` / `.Error(...)` / `.Debug(...)`.
2. Default `cfg.Logger = slog.Default()` so behaviour is unchanged
   for callers that don't care.

### Phase 6 — Make the scanner testable (1 day)

1. Introduce a `Fetcher` interface (`Fetch(ctx, url) (xml, error)`)
   so the sitemap parsing can be tested with fixtures, no network.
2. Add tests for `ScanSite` / `ScanSitemapIndex` using a fake
   `Fetcher` and an in-memory `Repository`.
3. Use `errgroup.WithContext` for the worker pool in
   `ScanSitemapIndex`, so the first error cancels the rest.

### Phase 7 — Naming + JSON tag cleanup (1 hour, breaking)

1. Rename `Url` → `URL`, `UrlParser` → `URLParser`, `ProxyUrl` →
   `ProxyURL`, `safeUrl` → `safeURL`, `sqlite_db` → `sqliteDB`,
   `site_sitemaps` → `siteSitemaps`.
2. Keep JSON tags as `"url"` to preserve the wire format, but use
   `URL` in Go (godoc shows `URL`, JSON stays the same).
3. Bump the module to `v1.0.0` in a single release with a CHANGELOG.

### Phase 8 — Documentation (half a day)

1. Rewrite `README.md`:
   - one-paragraph "what this is",
   - one code block "quickstart",
   - "configuration" table,
   - "architecture" 3-bullet summary,
   - "limitations" (single sitemap pass, no recursion beyond index,
     no `lastmod` extraction, etc.),
   - license + go.dev badge.
2. Add a `Example` test (Go example test) for `NewScanner` so it
   appears in pkg.go.dev.
3. Add `CHANGELOG.md`.

### Phase 9 — Optional: drop Colly + GORM (2 days, biggest win)

If the sitemap / robots use case is the **only** use case, the entire
Colly dependency is overkill. A ~150-line package using `net/http` +
`encoding/xml` would:

- remove three direct dependencies,
- make the code trivially testable with `httptest.Server`,
- remove the shared-collector footgun (F16),
- remove the dependency on the unmaintained `colly-sqlite3-storage`.

GORM can be replaced by `database/sql` + `modernc.org/sqlite` (pure Go,
no cgo) and a hand-rolled schema migration. If you keep GORM, fine —
but at least gate it behind the `Repository` interface (Phase 4).

---

## 6. Target shape after refactor

```
sitemap-go/
├── .github/workflows/ci.yml         (gofmt, vet, staticcheck, test -race)
├── CHANGELOG.md
├── LICENSE
├── README.md                        (quickstart, config, arch, limits)
├── doc.go                           (// Package sitemap ...)
├── go.mod                           (Go 1.22+, explicit toolchain)
├── go.sum
├── robots.go                        (was robots-txt.go)
├── scanner.go                       (was sitemap.go: Scanner + ScanSite + ScanSitemapIndex)
├── fetcher.go                       (Fetcher interface, HTTP impl)
├── errors.go                        (sentinel errors)
├── repo/
│   ├── doc.go
│   ├── repo.go                      (Repository interface)
│   └── sqliterepo/
│       ├── doc.go
│       ├── sqlite.go                (NewSQLite(ctx, path) (*gorm.DB, error))
│       └── data.go                  (impl of Repository)
├── types/
│   ├── doc.go
│   ├── domain.go                    (DomainFrom, URLToPathSlug, SafeURLParser)
│   ├── hash.go
│   ├── link.go
│   ├── site.go
│   └── sitemapIndex.go
├── config/
│   ├── doc.go
│   └── config.go                    (Config struct, DefaultConfig, Validate)
├── examples/
│   └── main.go                      (small, runnable, no stray binaries)
└── *_test.go everywhere
```

Public API (target):

```go
type Config struct { ... }
func DefaultConfig() Config { ... }

type Scanner struct{ ... }
func NewScanner(cfg *config.Config) (*Scanner, error)
func (s *Scanner) ScanSite(ctx context.Context, url string) error
func (s *Scanner) ScanSitemapIndex(ctx context.Context, limit int) error
func (s *Scanner) Close() error

type Repository interface { ... }
type Fetcher interface { ... }

var (
    ErrInvalidConfig = errors.New("invalid config")
    ErrNoSitemap     = errors.New("no sitemap found")
)
```

---

## 7. Quick-win checklist

If you only have an hour, do this and ship:

- [ ] `gofmt -w .`
- [ ] `rm examples/examples`  (the 24 MB ELF)
- [ ] Add `// Package ...` doc comments to all 5 packages
- [ ] Fix `staticcheck` ST1003 (naming) and ST1020 (doc comments) — these
  are mechanical and the `gofmt`-equivalent for naming
- [ ] Change `NewScanner` to `(*Scanner, error)` and update the example
- [ ] Replace `panic("stop scanning")` with `return err`
- [ ] Add `types/domain_test.go` and `types/hash_test.go` with
  table-driven tests (10 minutes)
- [ ] Add a `.github/workflows/ci.yml` that runs the four checks

That alone takes this from "prototype" to "shippable v0.2.0".

---

## 8. Appendix — full `staticcheck -checks=all` output

```
colly.go:1:1: at least one file in a package should have a package comment (ST1000)
config\config.go:1:1: at least one file in a package should have a package comment (ST1000)
config\config.go:5:2: struct field ProxyUrl should be ProxyURL (ST1003)
config\dataDir.go:1:1: at least one file in a package should have a package comment (ST1000)
robots-txt.go:1:1: at least one file in a package should have a package comment (ST1000)
robots-txt.go:21:2: var safeUrl should be safeURL (ST1003)
sitemap.go:1:1: at least one file in a package should have a package comment (ST1000)
sitemap.go:39:1: comment on exported method ScanSite should be of the form "ScanSite ..." (ST1020)
sitemap.go:73:1: comment on exported method ScanSitemapIndex should be of the form "ScanSitemapIndex ..." (ST1020)
sitemap.go:110:1: comment on exported method ExtractSitemapIndex should be of the form "ExtractSitemapIndex ..." (ST1020)
storage\data.go:1:1: at least one file in a package should have a package comment (ST1000)
storage\sqlite.go:1:1: at least one file in a package should have a package comment (ST1000)
storage\sqlite.go:18:2: should not use underscores in Go names; var sqlite_db should be sqliteDB (ST1003)
storage\storage.go:1:1: at least one file in a package should have a package comment (ST1000)
storage\tables.go:1:1: at least one file in a package should have a package comment (ST1000)
types\domain.go:1:1: at least one file in a package should have a package comment (ST1000)
types\domain.go:32:6: func SafeUrlParser should be SafeURLParser (ST1003)
types\hash.go:1:1: at least one file in a package should have a package comment (ST1000)
types\hash.go:22:1: comment on exported function GetHashIDs should be of the form "GetHashIDs ..." (ST1020)
types\link.go:1:1: at least one file in a package should have a package comment (ST1000)
types\link.go:7:2: struct field Url should be URL (ST1003)
types\site.go:1:1: at least one file in a package should have a package comment (ST1000)
types\site.go:7:2: struct field Url should be URL (ST1003)
types\sitemapIndex.go:1:1: at least one file in a package should have a package comment (ST1000)
types\sitemapIndex.go:6:2: struct field Url should be URL (ST1003)
types\sitemapIndex.go:23:2: should not use underscores in Go names; var site_sitemaps should be siteSitemaps (ST1003)
```
