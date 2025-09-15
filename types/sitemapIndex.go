package types

type SitemapIndex struct {
	Hash    string `json:"hash" gorm:"unique"`
	Domain  string `json:"domain"`
	Url     string `json:"url"`
	Scanned bool   `json:"scanned"`
}

func (s SitemapIndex) HashID() string {
	return s.Hash
}
func NewSitemapIndex(url string) *SitemapIndex {
	return &SitemapIndex{
		Hash:   Hash256(url),
		Domain: DomainFrom(url),
		Url:    url,
	}
}

func NewSitemapIndexsFrom(site *Site) []SitemapIndex {
	var sitemaps []SitemapIndex
	site_sitemaps := site.ParseSitemaps()
	if site_sitemaps == nil {
		return nil
	}
	for _, url := range site_sitemaps {
		newSitemap := NewSitemapIndex(url)
		sitemaps = append(sitemaps, *newSitemap)
	}

	return sitemaps
}
