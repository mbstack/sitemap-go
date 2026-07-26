package sitemap

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/mbstack/sitemap-go/config"
	"github.com/mbstack/sitemap-go/internal/sitemaperr"
	"github.com/mbstack/sitemap-go/internal/testutil"
)

func TestParseRobotsTxt_Empty(t *testing.T) {
	_, err := parseRobotsTxt([]byte(""))
	if !errors.Is(err, sitemaperr.ErrNoSitemap) {
		t.Fatalf("expected ErrNoSitemap, got %v", err)
	}
}

func TestParseRobotsTxt_OneSitemap(t *testing.T) {
	body := []byte("Sitemap: https://x.com/sitemap.xml\n")
	got, err := parseRobotsTxt(body)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got.Sitemaps) != 1 || got.Sitemaps[0] != "https://x.com/sitemap.xml" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseRobotsTxt_MultipleSitemaps(t *testing.T) {
	body := []byte(`User-agent: *
Disallow:

Sitemap: https://x.com/sitemap1.xml
Sitemap: https://x.com/sitemap2.xml
Sitemap: https://x.com/sitemap3.xml
`)
	got, err := parseRobotsTxt(body)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got.Sitemaps) != 3 {
		t.Fatalf("want 3, got %d (%+v)", len(got.Sitemaps), got.Sitemaps)
	}
}

func TestParseRobotsTxt_CaseInsensitive(t *testing.T) {
	body := []byte("SITEMAP: https://x.com/a.xml\nsitemap: https://x.com/b.xml\nSiteMap: https://x.com/c.xml\n")
	got, err := parseRobotsTxt(body)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got.Sitemaps) != 3 {
		t.Fatalf("want 3 (case-insensitive), got %d", len(got.Sitemaps))
	}
}

func TestParseRobotsTxt_SkipsComments(t *testing.T) {
	body := []byte(`# this is a comment
Sitemap: https://x.com/a.xml
# another comment
Sitemap: https://x.com/b.xml
`)
	got, _ := parseRobotsTxt(body)
	if len(got.Sitemaps) != 2 {
		t.Fatalf("want 2, got %d", len(got.Sitemaps))
	}
}

func TestParseRobotsTxt_InlineCommentStripped(t *testing.T) {
	body := []byte("Sitemap: https://x.com/a.xml # trailing comment\n")
	got, _ := parseRobotsTxt(body)
	if len(got.Sitemaps) != 1 || got.Sitemaps[0] != "https://x.com/a.xml" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseRobotsTxt_WhitespaceTolerated(t *testing.T) {
	body := []byte("  Sitemap:    https://x.com/a.xml   \n")
	got, _ := parseRobotsTxt(body)
	if len(got.Sitemaps) != 1 || got.Sitemaps[0] != "https://x.com/a.xml" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseRobotsTxt_EmptySitemapValueIgnored(t *testing.T) {
	body := []byte("Sitemap:\nSitemap: https://x.com/a.xml\n")
	got, _ := parseRobotsTxt(body)
	if len(got.Sitemaps) != 1 {
		t.Fatalf("want 1, got %d", len(got.Sitemaps))
	}
}

func TestGetSitemapURLs_HappyPath(t *testing.T) {
	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/robots.txt") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("Sitemap: https://x.com/sitemap.xml\n"))
	}))
	cfg := defaultTestConfig(t, srv.Client())
	got, err := GetSitemapURLs(context.Background(), srv.URL, cfg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got.Sitemaps) != 1 || got.Sitemaps[0] != "https://x.com/sitemap.xml" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetSitemapURLs_NoSitemap(t *testing.T) {
	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /admin\n"))
	}))
	cfg := defaultTestConfig(t, srv.Client())
	_, err := GetSitemapURLs(context.Background(), srv.URL, cfg)
	if !errors.Is(err, sitemaperr.ErrNoSitemap) {
		t.Fatalf("expected ErrNoSitemap, got %v", err)
	}
}

func TestGetSitemapURLs_HTTPError(t *testing.T) {
	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	cfg := defaultTestConfig(t, srv.Client())
	_, err := GetSitemapURLs(context.Background(), srv.URL, cfg)
	if !errors.Is(err, sitemaperr.ErrFetch) {
		t.Fatalf("expected ErrFetch, got %v", err)
	}
}

func TestGetSitemapURLs_NilContext(t *testing.T) {
	_, err := GetSitemapURLs(context.Background(), "https://x.com", config.Config{}) //nolint:staticcheck // intentional nil ctx
	if err == nil {
		t.Fatal("expected error for nil context")
	}
}

func TestGetSitemapURLs_BadURL(t *testing.T) {
	cfg := defaultTestConfig(t, &http.Client{})
	_, err := GetSitemapURLs(context.Background(), "not a url", cfg)
	if !errors.Is(err, sitemaperr.ErrFetch) {
		t.Fatalf("expected ErrFetch, got %v", err)
	}
}

func TestGetSitemapURLs_URLEmpty(t *testing.T) {
	cfg := defaultTestConfig(t, &http.Client{})
	_, err := GetSitemapURLs(context.Background(), "", cfg)
	if !errors.Is(err, sitemaperr.ErrFetch) {
		t.Fatalf("expected ErrFetch, got %v", err)
	}
}

func defaultTestConfig(t *testing.T, client *http.Client) config.Config {
	t.Helper()
	return config.Config{DataDir: t.TempDir(), HTTPClient: client}
}
