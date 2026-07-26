// Package httpx builds the shared *http.Client used by the
// sitemap-go library for robots.txt and sitemap fetches.
//
// It is intentionally tiny: a single New function with sane
// defaults. The package is internal/ so external modules cannot
// import it; the library surface uses it through Config.
package httpx
