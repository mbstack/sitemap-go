// Package ratelimit provides a polite, bounded random delay
// between HTTP requests. Used by the sitemap-go fetcher to be a
// well-behaved client.
//
// The package is internal/ so external modules cannot import it.
package ratelimit
