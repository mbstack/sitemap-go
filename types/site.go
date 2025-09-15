package types

import "encoding/json"

type Site struct {
	Domain    string `json:"domain" gorm:"unique"`
	Url       string `json:"url"`
	Sitemaps  string `json:"sitemaps" gorm:"type:text"` //sitemap indexs
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func NewSite(url string) *Site {
	return &Site{
		Domain: DomainFrom(url),
		Url:    url,
	}
}

func (s *Site) SetSitemaps(sitemaps []string) *Site {
	dataBytes, err := json.Marshal(sitemaps)
	if err != nil {
		return s
	}
	s.Sitemaps = string(dataBytes)
	return s
}
func (s *Site) ParseSitemaps() []string {
	var sitemaps []string
	if err := json.Unmarshal([]byte(s.Sitemaps), &sitemaps); err != nil {
		return nil
	}
	return sitemaps
}
