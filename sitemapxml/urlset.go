// Package sitemapxml: urlset parsing.
package sitemapxml

import (
	"encoding/xml"
	"fmt"
	"io"
)

// urlSet is the on-wire shape of a <urlset>.
type urlSet struct {
	XMLName xml.Name   `xml:"urlset"`
	URLs    []urlEntry `xml:"url"`
}

// urlEntry is one <url> inside a urlset.
type urlEntry struct {
	Loc        string `xml:"loc"`
	Lastmod    string `xml:"lastmod"`
	Changefreq string `xml:"changefreq"`
	Priority   string `xml:"priority"`
}

// URLEntry is one <url> from a <urlset> with the fields the
// sitemap spec defines for sitemaps. Lastmod, Changefreq, and
// Priority are optional; empty strings mean "not present".
type URLEntry struct {
	URL        string
	Lastmod    string
	Changefreq string
	Priority   string
}

// ParseURLSet parses a <urlset> document and returns every
// <url><loc>...</loc></url> entry in document order. Empty
// locations are skipped. Returns an empty (non-nil) slice for
// an empty document.
func ParseURLSet(r io.Reader) ([]URLEntry, error) {
	if r == nil {
		return nil, fmt.Errorf("sitemapxml: nil reader")
	}
	var us urlSet
	if err := xml.NewDecoder(r).Decode(&us); err != nil {
		return nil, fmt.Errorf("sitemapxml: decode urlset: %w", err)
	}
	out := make([]URLEntry, 0, len(us.URLs))
	for _, e := range us.URLs {
		if e.Loc == "" {
			continue
		}
		out = append(out, URLEntry{
			URL:        e.Loc,
			Lastmod:    e.Lastmod,
			Changefreq: e.Changefreq,
			Priority:   e.Priority,
		})
	}
	return out, nil
}
