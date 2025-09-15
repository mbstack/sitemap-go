package main

import (
	"github.com/MegaBytee/sitemap-go"
	"github.com/MegaBytee/sitemap-go/config"
)

func main() {

	cfg := config.Config{
		WithProxy: false,
		WithCache: true,
	}

	scanner := sitemap.NewScanner(&cfg)
	if scanner == nil {
		panic("stop here")
	}
	scanner.ScanSite("https://megabytee.com/")
	scanner.ScanSitemapIndex(100)

	scanner.Close()

}
