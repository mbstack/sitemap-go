package sitemap

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/MegaBytee/sitemap-go/config"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
	"github.com/velebak/colly-sqlite3-storage/colly/sqlite3"
)

func newCollyScrapper(cfg *config.Config) (*colly.Collector, error) {
	c := colly.NewCollector()

	dataDir, err := config.GetDataDir()
	if err != nil {
		return nil, err
	}
	newDirPath := filepath.Join(dataDir, "colly")
	// Check if the directory exists
	if _, err := os.Stat(newDirPath); os.IsNotExist(err) {
		// Create the directory
		err := os.Mkdir(newDirPath, os.ModePerm)
		if err != nil {
			return nil, err
		}

	}

	storageDir := filepath.Join(newDirPath, "sitemap.db")
	storage := &sqlite3.Storage{
		Filename: storageDir,
	}

	err = c.SetStorage(storage)
	if err != nil {
		return nil, err
	}
	if cfg.WithCache {
		cacheDir := filepath.Join(newDirPath, "sitemap")
		c.CacheDir = cacheDir
	}

	if cfg.WithProxy {
		err := c.SetProxy(cfg.ProxyUrl)
		if err != nil {
			return nil, err
		}
	}

	extensions.RandomUserAgent(c)
	extensions.Referer(c)
	c.OnRequest(func(r *colly.Request) {
		log.Println("visiting:", r.URL)
		setDelayInMs(10, 500)
	})

	c.OnResponse(func(r *colly.Response) {
		log.Println(r.Request.URL, "\t", r.StatusCode)
	})
	c.OnError(func(r *colly.Response, err error) {
		log.Println(r.Request.URL, "\t", r.StatusCode, "\nError:", err)
	})

	return c, nil
}
func setDelayInMs(x, y int) {
	delay := rand.Intn(y) + x // Random number between x and y
	fmt.Printf("Sleeping for %d ms before the next request...\n", delay)
	time.Sleep(time.Duration(delay) * time.Millisecond)
}
