package types

type Link struct {
	Hash         string `json:"hash" gorm:"unique"`
	Domain       string `json:"domain"`
	SitemapIndex string `json:"sitemap_index"`
	Url          string `json:"url"`
}

func (link Link) HashID() string {
	return link.Hash
}
func NewLink(url string) *Link {
	return &Link{
		Hash:   Hash256(url),
		Domain: DomainFrom(url),
		Url:    url,
	}
}

func NewLinksFromSitemapIndex(urls []string) []Link {
	var links []Link
	for _, url := range urls {
		newLink := NewLink(url)
		links = append(links, *newLink)
	}
	return links
}
