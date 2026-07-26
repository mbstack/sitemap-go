package sitemapxml_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mbstack/sitemap-go/sitemapxml"
)

func TestParseSitemapIndex_Empty(t *testing.T) {
	body := `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
</sitemapindex>`
	got, err := sitemapxml.ParseSitemapIndex(strings.NewReader(body))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want 0, got %d", len(got))
	}
}

func TestParseSitemapIndex_Single(t *testing.T) {
	body := `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap>
    <loc>https://x.com/sitemap1.xml</loc>
  </sitemap>
</sitemapindex>`
	got, err := sitemapxml.ParseSitemapIndex(strings.NewReader(body))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := []sitemapxml.SitemapIndexEntry{{URL: "https://x.com/sitemap1.xml"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %+v, got %+v", want, got)
	}
}

func TestParseSitemapIndex_Many(t *testing.T) {
	body := `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>https://x.com/a.xml</loc></sitemap>
  <sitemap><loc>https://x.com/b.xml</loc></sitemap>
  <sitemap><loc>https://x.com/c.xml</loc></sitemap>
</sitemapindex>`
	got, err := sitemapxml.ParseSitemapIndex(strings.NewReader(body))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3, got %d (%+v)", len(got), got)
	}
}

func TestParseSitemapIndex_SkipsEmptyLoc(t *testing.T) {
	body := `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc></loc></sitemap>
  <sitemap><loc>https://x.com/a.xml</loc></sitemap>
</sitemapindex>`
	got, _ := sitemapxml.ParseSitemapIndex(strings.NewReader(body))
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
}

func TestParseSitemapIndex_Malformed(t *testing.T) {
	body := `<not-closed>`
	if _, err := sitemapxml.ParseSitemapIndex(strings.NewReader(body)); err == nil {
		t.Fatal("expected error for malformed XML")
	}
}

func TestParseSitemapIndex_NilReader(t *testing.T) {
	if _, err := sitemapxml.ParseSitemapIndex(nil); err == nil {
		t.Fatal("expected error for nil reader")
	}
}

func TestParseURLSet_Empty(t *testing.T) {
	body := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
</urlset>`
	got, err := sitemapxml.ParseURLSet(strings.NewReader(body))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want 0, got %d", len(got))
	}
}

func TestParseURLSet_LocOnly(t *testing.T) {
	body := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://x.com/a</loc></url>
</urlset>`
	got, err := sitemapxml.ParseURLSet(strings.NewReader(body))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 || got[0].URL != "https://x.com/a" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseURLSet_FullEntry(t *testing.T) {
	body := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://x.com/a</loc>
    <lastmod>2026-07-26</lastmod>
    <changefreq>daily</changefreq>
    <priority>0.8</priority>
  </url>
</urlset>`
	got, err := sitemapxml.ParseURLSet(strings.NewReader(body))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := []sitemapxml.URLEntry{{
		URL:        "https://x.com/a",
		Lastmod:    "2026-07-26",
		Changefreq: "daily",
		Priority:   "0.8",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %+v, got %+v", want, got)
	}
}

func TestParseURLSet_Malformed(t *testing.T) {
	if _, err := sitemapxml.ParseURLSet(strings.NewReader(`<broken`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseURLSet_NilReader(t *testing.T) {
	if _, err := sitemapxml.ParseURLSet(nil); err == nil {
		t.Fatal("expected error")
	}
}
