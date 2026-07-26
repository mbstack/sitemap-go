package sitemap_test

import (
	"context"
	"fmt"

	"github.com/mbstack/sitemap-go"
	"github.com/mbstack/sitemap-go/config"
	"github.com/mbstack/sitemap-go/logger"
)

// ExampleNewScanner demonstrates the canonical library usage.
// The DataDir must exist; NewScanner creates it if missing.
func ExampleNewScanner() {
	cfg := config.DefaultConfig()
	cfg.DataDir = "./data" // required
	cfg.Logger = logger.New(logger.Options{Level: "info"})

	s, err := sitemap.NewScanner(context.Background(), cfg)
	if err != nil {
		fmt.Println("construct:", err)
		return
	}
	defer s.Close()

	if err := s.ScanSite(context.Background(), "https://example.com"); err != nil {
		fmt.Println("scan site:", err)
		return
	}
	if err := s.ScanSitemapIndex(context.Background(), 100); err != nil {
		fmt.Println("scan indexes:", err)
		return
	}
	fmt.Println("done")
}
