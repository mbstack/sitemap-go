// Package sqlite is the default gorm-backed SQLite Store.
package sqlite

import (
	"context"
	"errors"
	"sync"

	"github.com/mbstack/sitemap-go/internal/sitemaperr"
	"github.com/mbstack/sitemap-go/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// defaultBatchSize is the batch size used by CreateInBatches when
// callers do not specify one. Exposed via the package constant
// for tests.
const defaultBatchSize = 100

// models is the canonical list of gorm models owned by the
// store package. AutoMigrate uses it.
var models = []interface{}{
	&store.Site{},
	&store.SitemapIndex{},
	&store.Link{},
}

// Migrate runs AutoMigrate over the store models. Safe to call
// repeatedly; gorm's AutoMigrate is idempotent. It also enables
// WAL mode and a busy-timeout so concurrent writers do not
// deadlock on the default journal-mode lock.
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(models...); err != nil {
		return err
	}
	// Enable WAL and set a busy timeout. WAL allows concurrent
	// readers and a single writer; the busy timeout makes the
	// writer wait briefly for the lock to release instead of
	// failing immediately. See:
	// https://www.sqlite.org/wal.html
	if db.Dialector.Name() == "sqlite" {
		if err := db.Exec("PRAGMA journal_mode=WAL").Error; err != nil {
			return err
		}
		if err := db.Exec("PRAGMA busy_timeout=5000").Error; err != nil {
			return err
		}
	}
	return nil
}

// Store is the gorm-backed implementation of store.Store,
// backed by a SQLite database.
type Store struct {
	db   *gorm.DB
	owns bool
	once sync.Once
}

// New wraps an existing *gorm.DB in a Store. The caller retains
// ownership of db; Close on the Store does not close db.
func New(db *gorm.DB) *Store {
	return &Store{db: db, owns: false}
}

// NewOwned wraps an existing *gorm.DB in a Store and takes
// ownership: Close will close the underlying *sql.DB. Use this
// when the Store is the sole owner of the connection (e.g.
// NewScanner constructs its own DB).
func NewOwned(db *gorm.DB) *Store {
	return &Store{db: db, owns: true}
}

// SaveSite inserts s on conflict of Domain (DoNothing).
func (s *Store) SaveSite(ctx context.Context, site *store.Site) error {
	if site == nil {
		return sitemaperr.Wrap("SaveSite", errors.New("nil site"))
	}
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(site).Error
}

// GetSiteByDomain returns the row matching domain, or
// ErrSiteNotFound.
func (s *Store) GetSiteByDomain(ctx context.Context, domain string) (*store.Site, error) {
	var out store.Site
	err := s.db.WithContext(ctx).Where("domain = ?", domain).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, sitemaperr.ErrSiteNotFound
	}
	if err != nil {
		return nil, sitemaperr.Wrap("GetSiteByDomain", err)
	}
	return &out, nil
}

// CreateSitemapsInBatches inserts sitemaps in batches.
func (s *Store) CreateSitemapsInBatches(ctx context.Context, sitemaps []store.SitemapIndex) error {
	if len(sitemaps) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		CreateInBatches(&sitemaps, defaultBatchSize).Error
}

// GetSitemapIndexToScan returns up to limit unscanned sitemaps.
func (s *Store) GetSitemapIndexToScan(ctx context.Context, limit int) ([]store.SitemapIndex, error) {
	if limit <= 0 {
		limit = 100
	}
	var out []store.SitemapIndex
	err := s.db.WithContext(ctx).
		Where("scanned = ?", false).
		Order("id ASC").
		Limit(limit).
		Find(&out).Error
	if err != nil {
		return nil, sitemaperr.Wrap("GetSitemapIndexToScan", err)
	}
	return out, nil
}

// UpdateScannedSitemaps marks the rows whose Hash matches the
// hash of any entry in ss as scanned=true. Done in a single
// transaction with batched subqueries.
func (s *Store) UpdateScannedSitemaps(ctx context.Context, sitemaps []store.SitemapIndex) error {
	if len(sitemaps) == 0 {
		return nil
	}
	hashes := make([]string, 0, len(sitemaps))
	for _, s := range sitemaps {
		if s.Hash != "" {
			hashes = append(hashes, s.Hash)
		}
	}
	if len(hashes) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).
		Model(&store.SitemapIndex{}).
		Where("hash IN ?", hashes).
		Update("scanned", true).Error
}

// CreateLinksInBatches inserts links in batches.
func (s *Store) CreateLinksInBatches(ctx context.Context, links []store.Link) error {
	if len(links) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		CreateInBatches(&links, defaultBatchSize).Error
}

// Close releases the store. When the Store was constructed
// with NewOwned, Close also closes the underlying *sql.DB.
// When constructed with New, Close is a no-op.
func (s *Store) Close() error {
	var err error
	s.once.Do(func() {
		if !s.owns {
			return
		}
		sqlDB, derr := s.db.DB()
		if derr != nil {
			err = derr
			return
		}
		err = sqlDB.Close()
	})
	return err
}
