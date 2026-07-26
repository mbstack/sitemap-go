# Issue 0001 — `ScanSite` cannot distinguish a `<sitemapindex>` from a direct `<urlset>`

**Status:** Open
**Severity:** High — every site where `robots.txt` points to a single
`<urlset>` (no `<sitemapindex>` wrapper) will fail the
`ScanSitemapIndex` step with `parse urlset: expected element type
<urlset> but have <sitemapindex>`.
**Discovered:** 2026-07-26, during Phase 17 e2e testing
(`cmd/sitemap/main_test.go::TestRun_EndToEnd`).

---

## Summary

`ScanSite` assumes the URLs it finds in `robots.txt` are always
`<sitemapindex>` documents. It fetches each one, parses it as a
sitemapindex, and persists the inner `<sitemap><loc>` entries to the
`sitemap_indices` table. If a document is actually a `<urlset>`
(common for small sites that have only one), or if the fetch/parse
fails, the **parent URL** still ends up in `sitemap_indices` and
`ScanSitemapIndex` then tries to parse it as a `<urlset>`, which
fails.

The result: silent success on `ScanSite`, loud failure on
`ScanSitemapIndex` with no link rows persisted.

## Reproduction

### What the test does

`cmd/sitemap/main_test.go::TestRun_EndToEnd` starts an
`httptest.Server` that serves:

- `/robots.txt` → declares `Sitemap: <server>/sitemap-index.xml`
- `/sitemap-index.xml` → `<sitemapindex>` with two `<sitemap><loc>`
  entries pointing to `/sm1.xml` and `/sm2.xml`
- `/sm1.xml` and `/sm2.xml` → `<urlset>` documents with 2 and 1
  `<url>` entries respectively

### What actually happens

1. `ScanSite` fetches `/sitemap-index.xml`, parses it successfully,
   discovers 2 leaves (`/sm1.xml`, `/sm2.xml`).
2. `ScanSite` writes **2 rows** to `sitemap_indices` with the
   correct leaf URLs.
3. `ScanSitemapIndex` reads those 2 rows, fetches `/sm1.xml` and
   `/sm2.xml`, parses them as urlsets — **succeeds** for these.

So far so good. But here's the bug: the failing test output we
captured was from an *earlier* state of the code, before my fix,
where `ScanSite` was persisting the **parent** index URL
(`/sitemap-index.xml`) to `sitemap_indices`. After the fix, the
leaves are persisted. The two situations are confused below; the
**current state on `main`** is the latter (leaves) but the
**underlying design flaw** is the same and exposes itself in the
adjacent cases below.

### Adjacent case that is still broken (after the partial fix)

If a site's `robots.txt` points directly to a `<urlset>` (no
sitemapindex), the parent URL is the only URL the scanner knows
about. There are no leaves to persist. `ScanSitemapIndex` then
fetches the same URL, tries to parse it as a urlset — succeeds —
but the parent URL is the wrong row type.

## Root cause

File: `scanner.go`, function `ScanSite` (around lines 90–160).

The current code makes a single assumption about the documents
pointed to by `robots.txt`:

```go
for _, smURL := range robots.Sitemaps {
    s.delay.Wait()
    if err := ctx.Err(); err != nil { return err }
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
```

The bug is at the parser level: the code tries `ParseSitemapIndex`
unconditionally. If the document is actually a `<urlset>`, the
parse returns an error, the entry is logged-and-skipped, and the
result is the same `nil` return value as the happy path (the
sitemaps table ends up with 0 rows for that URL). On the next
`ScanSitemapIndex` call the table is empty and nothing happens.

Conversely, if the parser returns 0 entries for any other reason
(malformed XML, transient error), the result is also a `nil`
`leafURLs` followed by `ErrNoSitemap` — which is technically the
right error in the first case but wrong in the second.

There is also a related but distinct issue: there is **no
detection** of a `<urlset>` that comes directly from `robots.txt`.
A correct sitemap-walker should attempt both parses (index, then
urlset as fallback) when the first fails.

## Why the unit tests didn't catch this

`TestScanSite_HappyPath` in `scanner_test.go` and
`TestScanSitemapIndex_HappyPath` only exercise the case where
`<sitemapindex>` contains `<sitemap><loc>` entries. They never
test the "robots.txt points directly to a `<urlset>`" case, and
they never test the "sitemapindex parse fails" case. The e2e
test that surfaced this was added in Phase 17 specifically to
verify the full CLI pipeline end-to-end; the unit tests covered
the library functions but not their composition.

## Suggested fix

In `ScanSite`, after a fetch+parse failure, **fall back to
parsing the same body as a `<urlset>`**. If that succeeds, treat
the URL itself as a leaf (no further recursion needed). Example
sketch:

```go
for _, smURL := range robots.Sitemaps {
    // ... fetch ...
    entries, err := sitemapxml.ParseSitemapIndex(bytesReader(body))
    if err == nil {
        // sitemapindex path: collect leaves
        for _, e := range entries {
            leafURLs = append(leafURLs, e.URL)
        }
        continue
    }
    // Fallback: try urlset
    urlEntries, urlErr := sitemapxml.ParseURLSet(bytesReader(body))
    if urlErr == nil && len(urlEntries) > 0 {
        leafURLs = append(leafURLs, smURL) // the URL itself is a leaf
        continue
    }
    // Both parses failed: log and skip
    s.log.Warn().Err(err).Str("url", smURL).Msg("parse sitemap failed, skipping")
}
```

This requires the body to be re-readable. `bytes.Reader` is
seekable; capture the body in a `[]byte` (or a `bytes.Reader`) and
pass it to both parsers.

A cleaner long-term shape is to introduce a `sitemapxml.Probe`
function that returns one of three outcomes (`Index`, `URLSet`,
`Unknown`) and have the parser dispatch accordingly. That belongs
in a follow-up issue; the inline fallback above is enough for
v1.0.1.

## Tests to add

In `scanner_test.go` (or a new `scanner_integration_test.go`):

1. `TestScanSite_RobotsPointsToURLSet` — `robots.txt` points
   directly to a `<urlset>`. `ScanSite` should persist 1 row in
   `sitemap_indices` (the urlset URL itself).
2. `TestScanSite_RobotsPointsToBadXML` — `robots.txt` points to
   a URL that returns malformed XML. `ScanSite` should return
   `ErrNoSitemap` and not panic.
3. `TestScanSite_MixedShapes` — `robots.txt` declares 3 sitemaps:
   one `<sitemapindex>`, one `<urlset>`, one 404. `ScanSite`
   should succeed and persist 1 row for the sitemapindex (its
   single leaf) and 1 row for the urlset.
4. `cmd/sitemap/main_test.go::TestRun_EndToEnd` should be
   updated to use a mixed shape so the bug cannot regress.

## Affected files

- `scanner.go` — fix the fetch loop
- `scanner_test.go` — add the three tests above
- `cmd/sitemap/main_test.go` — keep the e2e but switch to mixed
  shape so it would have caught the original bug
- `CHANGELOG.md` — note the v1.0.1 fix

## Estimated effort

- Fix: ~15 lines in `scanner.go`
- Tests: ~50 lines, three new test cases
- Verification: full `go test ./... -race` should pass

## Workaround until the fix lands

If you must test against a real site today, the CLI works
correctly when:

- The site's `robots.txt` declares a `<sitemapindex>` (not a
  direct `<urlset>`), AND
- The sitemapindex itself parses successfully on the first try.

For most production sites (which use `<sitemapindex>`) this is
fine. For small static sites that point directly to a
`<urlset>`, the CLI will fail at `ScanSitemapIndex` time.

If you hit this, the manual workaround is to point the CLI at
the `<urlset>` URL directly (skipping `ScanSite`):

```
# Not implemented; would require a new CLI flag.
```

The recommended action is to fix the issue before tagging v1.0.0
or to ship v1.0.0 with a known-issue note and a v1.0.1 fix
shortly after.
