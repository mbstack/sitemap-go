package sitemap

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/MegaBytee/sitemap-go/config"
	"github.com/MegaBytee/sitemap-go/storage"
	"github.com/MegaBytee/sitemap-go/types"
	"github.com/gocolly/colly/v2"
)

type Scanner struct {
	c    *colly.Collector
	data *storage.Storage
}

func NewScanner(cfg *config.Config) *Scanner {
	c, err := newCollyScrapper(cfg)
	if err != nil {
		return nil
	}
	data := storage.New().Config()
	if data == nil {
		return nil
	}
	return &Scanner{
		c:    c,
		data: data,
	}
}

func (s *Scanner) Close() {
	s.data.Close()
}

// scan site sitemaps
func (s *Scanner) ScanSite(url string) {
	start := time.Now()

	newSite := types.NewSite(url)
	robotsTxt, err := GetSitemapUrlsFromRobotsTxt(url)
	if err != nil {
		panic("stop scanning")
	}

	var sitemaps []string
	for _, url := range robotsTxt.Sitemaps {
		urls := s.ExtractSitemapIndex(url)
		sitemaps = append(sitemaps, urls...)
	}
	log.Println("sitemaps found=:", len(sitemaps))
	newSite.SetSitemaps(sitemaps)

	err = s.data.SaveSite(newSite)
	if err != nil {
		log.Printf("something went wrong:err %v", err)
	}

	newSitemaps := types.NewSitemapIndexsFrom(newSite)

	err = s.data.CreateSitemapsInBatches(newSitemaps)
	if err != nil {
		log.Printf("something went wrong:err %v", err)
	}
	elapsed := time.Since(start)

	log.Println("ScanSite executed in :", elapsed)
}

// scan sitemapindex and extract links
func (s *Scanner) ScanSitemapIndex(limit int) {
	start := time.Now()
	sitemaps := s.data.GetSitemapIndexToScan(limit)
	if len(sitemaps) > 0 {
		// Create a WaitGroup to wait for all goroutines to finish
		var wg sync.WaitGroup
		// Create a buffered channel to limit concurrency
		sem := make(chan struct{}, 100) // Limit to 10 concurrent goroutines

		for _, index := range sitemaps {
			wg.Add(1) // Increment the WaitGroup counter

			go func(index types.SitemapIndex) {
				defer wg.Done()   // Notify that this goroutine is done
				sem <- struct{}{} // Acquire a token

				//extract links from this sitemap index url
				urls := s.GetLinksFromSitemapIndex(index.Url)
				if len(urls) > 0 {
					links := types.NewLinksFromSitemapIndex(urls)
					s.data.CreateLinksInBatches(links)
				}

				<-sem // Release the token
			}(index)
		}

		// Wait for all goroutines to finish
		wg.Wait()
		s.data.UpdateScannedSitemaps(sitemaps)
	}
	elapsed := time.Since(start)

	log.Println("ScanSitemapIndex executed in :", elapsed)
}

// extract more sitemapIndex from a sitemapIndex if has any
func (s *Scanner) ExtractSitemapIndex(url string) []string {
	var urls []string
	s.c.OnXML("//sitemapindex/sitemap", func(e *colly.XMLElement) {
		//println("onXLM/sitemapindex", e.Text)
		//loc = e.Text
		url := e.ChildText("loc")
		log.Println("Found Sitemap URL:", url)
		urls = append(urls, url)

	})
	err := s.c.Visit(url)
	if err != nil {
		log.Printf("something went wrong:err %v", err)
		return nil
	}

	return urls
}

func (s *Scanner) GetLinksFromSitemapIndex(url string) []string {
	var urls []string
	s.c.OnXML("//urlset/url", func(e *colly.XMLElement) {

		url := e.ChildText("loc")
		fmt.Println("Found URL:", url)
		urls = append(urls, url)

	})

	err := s.c.Visit(url)
	if err != nil {
		log.Printf("something went wrong:err %v", err)
		return nil
	}
	return urls
}
