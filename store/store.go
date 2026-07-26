// Package store defines the persistence contract for a Scanner.
package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
)

// Site is a row in the sites table. Sitemaps is a JSON-encoded
// list of sitemapindex URLs discovered for this site; the
// Scanner extracts it to typed values via ParseSitemaps.
type Site struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Domain    string `gorm:"uniqueIndex;not null" json:"domain"`
	URL       string `gorm:"not null" json:"url"`
	Sitemaps  string `gorm:"type:text" json:"sitemaps"`
	CreatedAt int64  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt int64  `gorm:"autoUpdateTime" json:"updated_at"`
}

// SitemapIndex is a row in the sitemaps table. It is one
// <sitemap><loc>...</loc></sitemap> entry extracted from a
// sitemapindex.
type SitemapIndex struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	Hash    string `gorm:"uniqueIndex;not null" json:"hash"`
	Domain  string `gorm:"index" json:"domain"`
	URL     string `gorm:"not null" json:"url"`
	Scanned bool   `gorm:"default:false;index" json:"scanned"`
}

// Link is a row in the links table. It is one <loc>...</loc>
// entry extracted from a <urlset>.
type Link struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Hash         string `gorm:"uniqueIndex;not null" json:"hash"`
	Domain       string `gorm:"index" json:"domain"`
	SitemapIndex string `gorm:"index" json:"sitemap_index"`
	URL          string `gorm:"not null" json:"url"`
	Lastmod      string `gorm:"index" json:"lastmod,omitempty"`
	Changefreq   string `json:"changefreq,omitempty"`
	Priority     string `json:"priority,omitempty"`
}

// HashOf returns the canonical sha256 hex hash used to identify
// a sitemap or a link. The same input always returns the same
// hash; the hash is what gives the table its unique index.
func HashOf(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// Store is the persistence contract used by Scanner.
//
// Implementations must be safe for concurrent use. All methods
// honour the context for cancellation; ctx.Err() is returned
// when the context is cancelled.
type Store interface {
	// SaveSite inserts (or updates on conflict) a site row.
	// Returns the row on success.
	SaveSite(ctx context.Context, s *Site) error

	// GetSiteByDomain returns the Site whose Domain column
	// matches domain, or ErrSiteNotFound when no row exists.
	GetSiteByDomain(ctx context.Context, domain string) (*Site, error)

	// CreateSitemapsInBatches inserts sitemaps in batches of
	// batchSize. Existing rows (matched by Hash) are left
	// untouched (OnConflict DoNothing).
	CreateSitemapsInBatches(ctx context.Context, ss []SitemapIndex) error

	// GetSitemapIndexToScan returns up to limit sitemap rows
	// whose Scanned flag is false, ordered by ID.
	GetSitemapIndexToScan(ctx context.Context, limit int) ([]SitemapIndex, error)

	// UpdateScannedSitemaps marks every SitemapIndex in ss as
	// scanned=true. Matched by Hash.
	UpdateScannedSitemaps(ctx context.Context, ss []SitemapIndex) error

	// CreateLinksInBatches inserts links in batches of
	// batchSize. Existing rows are left untouched.
	CreateLinksInBatches(ctx context.Context, ls []Link) error

	// Close releases underlying resources. Safe to call once;
	// subsequent calls are no-ops.
	Close() error
}
