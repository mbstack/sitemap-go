// Package sitemapxml parses sitemap index and urlset XML
// documents into typed Go values. It performs no I/O: callers
// pass an io.Reader. This makes the package trivially testable
// with strings.NewReader and friendly to the Fetcher
// abstraction used by Scanner.
package sitemapxml
