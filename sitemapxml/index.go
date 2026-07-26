// Package sitemapxml parses sitemap index and urlset documents.
package sitemapxml

import (
	"encoding/xml"
	"fmt"
	"io"
)

// sitemapIndex is the on-wire shape of a <sitemapindex>.
type sitemapIndex struct {
	XMLName  xml.Name            `xml:"sitemapindex"`
	Sitemaps []sitemapIndexEntry `xml:"sitemap"`
}

// sitemapIndexEntry is one <sitemap> inside a sitemapindex.
type sitemapIndexEntry struct {
	Loc string `xml:"loc"`
}

// SitemapIndexEntry is one <sitemap><loc> from a sitemapindex.
type SitemapIndexEntry struct {
	URL string
}

// ParseSitemapIndex parses a <sitemapindex> document and returns
// every <loc> it contains, in document order. Empty locations
// are skipped. Returns an empty (non-nil) slice for an empty
// document.
func ParseSitemapIndex(r io.Reader) ([]SitemapIndexEntry, error) {
	if r == nil {
		return nil, fmt.Errorf("sitemapxml: nil reader")
	}
	var idx sitemapIndex
	if err := xml.NewDecoder(r).Decode(&idx); err != nil {
		return nil, fmt.Errorf("sitemapxml: decode sitemapindex: %w", err)
	}
	out := make([]SitemapIndexEntry, 0, len(idx.Sitemaps))
	for _, e := range idx.Sitemaps {
		if e.Loc != "" {
			out = append(out, SitemapIndexEntry{URL: e.Loc})
		}
	}
	return out, nil
}
