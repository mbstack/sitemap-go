package sitemap

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/mbstack/sitemap-go/config"
	"github.com/mbstack/sitemap-go/internal/ratelimit"
	"github.com/mbstack/sitemap-go/internal/sitemaperr"
	"github.com/mbstack/sitemap-go/internal/testutil"
	"github.com/mbstack/sitemap-go/store"
	"github.com/mbstack/sitemap-go/store/sqlite"
	"gorm.io/gorm"
)

// newTestScanner wires a real sqlite-backed store to an
// in-memory DB and a real CollyFetcher.
func newTestScanner(t *testing.T) (*Scanner, *gorm.DB) {
	t.Helper()
	db := testutil.NewTestDB(t)
	lg, _ := testutil.NewTestLogger(t)

	cfg := config.DefaultConfig()
	cfg.Logger = lg
	cfg.DataDir = t.TempDir()
	cfg.Concurrency = 4
	cfg.MinDelay = 0
	cfg.MaxDelay = 0
	cfg.UserAgent = "mbstack-test/1.0"

	f, err := NewCollyFetcher(cfg, lg)
	if err != nil {
		t.Fatalf("NewCollyFetcher: %v", err)
	}

	st := sqlite.New(db)
	s := &Scanner{
		cfg:   cfg,
		log:   lg,
		store: st,
		fetch: f,
		delay: ratelimit.Limiter{},
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, db
}

// --- Tests ---

func TestScanSite_HappyPath(t *testing.T) {
	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		switch r.URL.Path {
		case "/robots.txt":
			_, _ = w.Write([]byte("Sitemap: " + base + "/sitemap-index.xml\n"))
		case "/sitemap-index.xml":
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>` + base + `/sm1.xml</loc></sitemap>
  <sitemap><loc>` + base + `/sm2.xml</loc></sitemap>
</sitemapindex>`))
		default:
			http.NotFound(w, r)
		}
	}))
	s, db := newTestScanner(t)
	if err := s.ScanSite(context.Background(), srv.URL); err != nil {
		t.Fatalf("ScanSite: %v", err)
	}
	var siteCount int64
	if err := db.Model(&store.Site{}).Count(&siteCount).Error; err != nil {
		t.Fatalf("count sites: %v", err)
	}
	if siteCount != 1 {
		t.Fatalf("want 1 site, got %d", siteCount)
	}
	var smCount int64
	if err := db.Model(&store.SitemapIndex{}).Count(&smCount).Error; err != nil {
		t.Fatalf("count sitemaps: %v", err)
	}
	// ScanSite stores one row per leaf <sitemap><loc> extracted
	// from the sitemapindex. The test's sitemapindex has 2
	// entries, so 2 rows.
	if smCount != 2 {
		t.Fatalf("want 2 leaf sitemapindex rows, got %d", smCount)
	}
}

func TestScanSite_NoSitemap(t *testing.T) {
	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			_, _ = w.Write([]byte("User-agent: *\n"))
			return
		}
		http.NotFound(w, r)
	}))
	s, _ := newTestScanner(t)
	err := s.ScanSite(context.Background(), srv.URL)
	if !errors.Is(err, sitemaperr.ErrNoSitemap) {
		t.Fatalf("expected ErrNoSitemap, got %v", err)
	}
}

func TestScanSite_404Robots(t *testing.T) {
	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	s, _ := newTestScanner(t)
	err := s.ScanSite(context.Background(), srv.URL)
	if !errors.Is(err, sitemaperr.ErrFetch) {
		t.Fatalf("expected ErrFetch, got %v", err)
	}
}

func TestScanSite_BadURL(t *testing.T) {
	s, _ := newTestScanner(t)
	err := s.ScanSite(context.Background(), "not a url")
	if !errors.Is(err, sitemaperr.ErrFetch) {
		t.Fatalf("expected ErrFetch, got %v", err)
	}
}

func TestScanSitemapIndex_HappyPath(t *testing.T) {
	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		switch r.URL.Path {
		case "/sm1.xml":
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>` + base + `/a</loc></url>
  <url><loc>` + base + `/b</loc></url>
</urlset>`))
		case "/sm2.xml":
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>` + base + `/c</loc></url>
</urlset>`))
		default:
			http.NotFound(w, r)
		}
	}))
	s, db := newTestScanner(t)
	sm1 := store.SitemapIndex{Hash: store.HashOf(srv.URL + "/sm1.xml"), URL: srv.URL + "/sm1.xml"}
	sm2 := store.SitemapIndex{Hash: store.HashOf(srv.URL + "/sm2.xml"), URL: srv.URL + "/sm2.xml"}
	if err := s.store.CreateSitemapsInBatches(context.Background(), []store.SitemapIndex{sm1, sm2}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := s.ScanSitemapIndex(context.Background(), 10); err != nil {
		t.Fatalf("ScanSitemapIndex: %v", err)
	}
	var linkCount int64
	if err := db.Model(&store.Link{}).Count(&linkCount).Error; err != nil {
		t.Fatalf("count links: %v", err)
	}
	if linkCount != 3 {
		t.Fatalf("want 3 links, got %d", linkCount)
	}
}

func TestScanSitemapIndex_Empty(t *testing.T) {
	s, _ := newTestScanner(t)
	if err := s.ScanSitemapIndex(context.Background(), 10); err != nil {
		t.Fatalf("expected nil on empty, got %v", err)
	}
}

func TestScanSitemapIndex_MarksScanned(t *testing.T) {
	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>` + base + `/a</loc></url>
</urlset>`))
	}))
	s, _ := newTestScanner(t)
	sm := store.SitemapIndex{Hash: store.HashOf(srv.URL + "/sm.xml"), URL: srv.URL + "/sm.xml"}
	if err := s.store.CreateSitemapsInBatches(context.Background(), []store.SitemapIndex{sm}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := s.ScanSitemapIndex(context.Background(), 10); err != nil {
		t.Fatalf("ScanSitemapIndex: %v", err)
	}
	row, err := s.store.GetSitemapIndexToScan(context.Background(), 10)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(row) != 0 {
		t.Fatalf("sitemap should be marked scanned, got %d unscanned", len(row))
	}
}

func TestScanSitemapIndex_OneFailsPropagates(t *testing.T) {
	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		if r.URL.Path == "/sm1.xml" {
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>` + base + `/a</loc></url>
</urlset>`))
			return
		}
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	s, _ := newTestScanner(t)
	sm1 := store.SitemapIndex{Hash: store.HashOf(srv.URL + "/sm1.xml"), URL: srv.URL + "/sm1.xml"}
	sm2 := store.SitemapIndex{Hash: store.HashOf(srv.URL + "/sm2.xml"), URL: srv.URL + "/sm2.xml"}
	if err := s.store.CreateSitemapsInBatches(context.Background(), []store.SitemapIndex{sm1, sm2}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := s.ScanSitemapIndex(context.Background(), 10); err == nil {
		t.Fatal("expected an error from one failing sitemap")
	}
}

func TestNewScanner_RequiresFields(t *testing.T) {
	cfg := config.DefaultConfig() // no DataDir, no Logger
	_, err := NewScanner(context.Background(), cfg)
	if !errors.Is(err, sitemaperr.ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestNewScanner_BuildsDefaultStoreAndFetcher(t *testing.T) {
	lg, _ := testutil.NewTestLogger(t)
	cfg := config.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.Logger = lg
	s, err := NewScanner(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewScanner: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal("Close should not error")
	}
}

func TestScanner_CloseIdempotent(t *testing.T) {
	s, _ := newTestScanner(t)
	if err := s.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second close should be safe: %v", err)
	}
}

func TestResolveSiteURL(t *testing.T) {
	got, err := ResolveSiteURL("https://x.com/path?q=1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "https://x.com" {
		t.Fatalf("got %q", got)
	}
	if _, err := ResolveSiteURL("not a url"); err == nil {
		t.Fatal("expected error")
	}
}

func TestDomainOf(t *testing.T) {
	if got := DomainOf("https://x.com/path"); got != "x.com" {
		t.Fatalf("got %q", got)
	}
	if got := DomainOf("not a url"); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}
