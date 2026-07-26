package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/mbstack/sitemap-go/internal/testutil"
	"github.com/mbstack/sitemap-go/store"
	"github.com/mbstack/sitemap-go/store/sqlite"
	"gorm.io/gorm"
)

func TestRun_HelpFlag(t *testing.T) {
	code, err := run([]string{"-h"}, os.Stdout, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}
}

func TestRun_NoArgs(t *testing.T) {
	code, _ := run([]string{}, os.Stdout, os.Stderr)
	if code != 0 {
		t.Fatalf("want exit 0 on no-args (prints usage), got %d", code)
	}
}

func TestRun_UnknownSubcommand(t *testing.T) {
	code, err := run([]string{"frobnicate"}, os.Stdout, os.Stderr)
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	if code != 1 {
		t.Fatalf("want exit 1, got %d", code)
	}
}

func TestRun_ScanMissingURL(t *testing.T) {
	code, err := run([]string{"scan"}, os.Stdout, os.Stderr)
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
	if code != 1 {
		t.Fatalf("want exit 1, got %d", code)
	}
}

func TestRun_ScanInvalidURL(t *testing.T) {
	dir := t.TempDir()
	args := []string{
		"scan", "not a url",
		"--data", dir,
		"--log-level", "error",
	}
	code, err := run(args, os.Stdout, os.Stderr)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if code != 2 {
		t.Fatalf("want exit 2, got %d", code)
	}
}

// TestRun_EndToEnd is the real integration test: spin up an
// httptest server with a full sitemap layout, run the CLI
// against it, then read the SQLite DB to verify the rows
// landed correctly.
func TestRun_EndToEnd(t *testing.T) {
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
		case "/sm1.xml":
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>` + base + `/page-a</loc><lastmod>2026-07-26</lastmod></url>
  <url><loc>` + base + `/page-b</loc></url>
</urlset>`))
		case "/sm2.xml":
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>` + base + `/page-c</loc></url>
</urlset>`))
		default:
			http.NotFound(w, r)
		}
	}))

	dir := filepath.Join(t.TempDir(), "data")
	t.Logf("data dir: %s", dir)
	t.Logf("server URL: %s", srv.URL)
	args := []string{
		"scan", srv.URL,
		"--data", dir,
		"--log-level", "error",
		"--min-delay", "0",
		"--max-delay", "0",
		"--limit", "10",
	}
	code, err := run(args, os.Stdout, os.Stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}

	// Verify the rows landed in the CLI's data dir.
	db, err := sqlite.Open(filepath.Join(dir, "sitemap.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := sqlite.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Print sitemap URLs for debugging.
	var rows []store.SitemapIndex
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("query: %v", err)
	}
	for _, r := range rows {
		t.Logf("sitemap_index: url=%s hash=%s", r.URL, r.Hash)
	}
	if err := assertRows(db); err != nil {
		t.Fatalf("assert: %v", err)
	}
}

func assertRows(db *gorm.DB) error {
	var sites int64
	if err := db.Model(&store.Site{}).Count(&sites).Error; err != nil {
		return err
	}
	if sites != 1 {
		return fmt.Errorf("sites: want 1, got %d", sites)
	}
	var indexes int64
	if err := db.Model(&store.SitemapIndex{}).Count(&indexes).Error; err != nil {
		return err
	}
	if indexes != 2 {
		return fmt.Errorf("sitemapindex rows: want 2, got %d", indexes)
	}
	var links int64
	if err := db.Model(&store.Link{}).Count(&links).Error; err != nil {
		return err
	}
	if links != 3 {
		return fmt.Errorf("link rows: want 3, got %d", links)
	}
	return nil
}

