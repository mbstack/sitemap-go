package storage

import (
	"github.com/MegaBytee/sitemap-go/types"
	"gorm.io/gorm/clause"
)

func (s *Storage) SaveSite(site *types.Site) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.Data.Clauses(clause.OnConflict{DoNothing: true}).Create(site).Error
}

func (s *Storage) CreateSitemapsInBatches(sitemap []types.SitemapIndex) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.Data.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(&sitemap, 100).Error

}

func (s *Storage) GetSitemapIndexToScan(limit int) []types.SitemapIndex {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sitemaps []types.SitemapIndex
	s.Data.Limit(limit).Where("scanned=?", false).Find(&sitemaps)
	return sitemaps
}
func (s *Storage) UpdateScannedSitemaps(sitemaps []types.SitemapIndex) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return updateOneFieldInBatches(s.Data,
		&types.SitemapIndex{}, "hash IN ?",
		types.GetHashIDs(sitemaps), "scanned", true, 100)
}

func (s *Storage) CreateLinksInBatches(links []types.Link) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.Data.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(&links, 100).Error

}
