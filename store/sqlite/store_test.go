package sqlite_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/mbstack/sitemap-go/internal/sitemaperr"
	"github.com/mbstack/sitemap-go/internal/testutil"
	"github.com/mbstack/sitemap-go/store"
	"github.com/mbstack/sitemap-go/store/sqlite"
)

func newStore(t *testing.T) *sqlite.Store {
	t.Helper()
	db := testutil.NewTestDB(t)
	return sqlite.New(db)
}

func TestSaveSite_GetSiteByDomain(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	in := &store.Site{Domain: "example.com", URL: "https://example.com/"}
	if err := s.SaveSite(ctx, in); err != nil {
		t.Fatalf("SaveSite: %v", err)
	}

	out, err := s.GetSiteByDomain(ctx, "example.com")
	if err != nil {
		t.Fatalf("GetSiteByDomain: %v", err)
	}
	if out.Domain != "example.com" || out.URL != "https://example.com/" {
		t.Fatalf("round-trip mismatch: %+v", out)
	}
}

func TestGetSiteByDomain_NotFound(t *testing.T) {
	s := newStore(t)
	_, err := s.GetSiteByDomain(context.Background(), "missing.example")
	if !errors.Is(err, sitemaperr.ErrSiteNotFound) {
		t.Fatalf("expected ErrSiteNotFound, got %v", err)
	}
}

func TestSaveSite_DuplicateDomainIsNoop(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if err := s.SaveSite(ctx, &store.Site{Domain: "x.com", URL: "https://x.com/"}); err != nil {
		t.Fatalf("first save: %v", err)
	}
	// Second save with same domain must not error and must not
	// overwrite the original row.
	if err := s.SaveSite(ctx, &store.Site{Domain: "x.com", URL: "https://x.com/different"}); err != nil {
		t.Fatalf("second save: %v", err)
	}
	out, err := s.GetSiteByDomain(ctx, "x.com")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if out.URL != "https://x.com/" {
		t.Fatalf("URL was overwritten: %s", out.URL)
	}
}

func TestCreateSitemapsInBatches_DeDup(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	a := store.SitemapIndex{Hash: store.HashOf("a"), Domain: "x.com", URL: "https://x.com/a.xml"}
	if err := s.CreateSitemapsInBatches(ctx, []store.SitemapIndex{a, a}); err != nil {
		t.Fatalf("CreateSitemapsInBatches: %v", err)
	}
	got, err := s.GetSitemapIndexToScan(ctx, 10)
	if err != nil {
		t.Fatalf("GetSitemapIndexToScan: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 row after dedup, got %d", len(got))
	}
}

func TestGetSitemapIndexToScan_OnlyUnscanned(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	mk := func(i int) store.SitemapIndex {
		u := "https://x.com/" + strconv.Itoa(i) + ".xml"
		return store.SitemapIndex{Hash: store.HashOf(u), Domain: "x.com", URL: u}
	}
	in := []store.SitemapIndex{mk(1), mk(2), mk(3), mk(4), mk(5)}
	if err := s.CreateSitemapsInBatches(ctx, in); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Mark the first two scanned.
	if err := s.UpdateScannedSitemaps(ctx, in[:2]); err != nil {
		t.Fatalf("update scanned: %v", err)
	}
	got, err := s.GetSitemapIndexToScan(ctx, 10)
	if err != nil {
		t.Fatalf("scan list: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 unscanned, got %d", len(got))
	}
}

func TestGetSitemapIndexToScan_HonoursLimit(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	mk := func(i int) store.SitemapIndex {
		u := "https://x.com/" + strconv.Itoa(i) + ".xml"
		return store.SitemapIndex{Hash: store.HashOf(u), Domain: "x.com", URL: u}
	}
	in := make([]store.SitemapIndex, 0, 250)
	for i := 0; i < 250; i++ {
		in = append(in, mk(i))
	}
	if err := s.CreateSitemapsInBatches(ctx, in); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := s.GetSitemapIndexToScan(ctx, 50)
	if err != nil {
		t.Fatalf("scan list: %v", err)
	}
	if len(got) != 50 {
		t.Fatalf("limit: want 50, got %d", len(got))
	}
}

func TestUpdateScannedSitemaps_BatchAll(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	mk := func(i int) store.SitemapIndex {
		u := "https://x.com/" + strconv.Itoa(i) + ".xml"
		return store.SitemapIndex{Hash: store.HashOf(u), Domain: "x.com", URL: u}
	}
	in := make([]store.SitemapIndex, 0, 250)
	for i := 0; i < 250; i++ {
		in = append(in, mk(i))
	}
	if err := s.CreateSitemapsInBatches(ctx, in); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := s.UpdateScannedSitemaps(ctx, in); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := s.GetSitemapIndexToScan(ctx, 1000)
	if err != nil {
		t.Fatalf("scan list: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 unscanned after bulk update, got %d", len(got))
	}
}

func TestCreateLinksInBatches_DeDup(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	a := store.Link{Hash: store.HashOf("https://x.com/page"), Domain: "x.com", URL: "https://x.com/page"}
	if err := s.CreateLinksInBatches(ctx, []store.Link{a, a}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// We don't have a GetLinks method in the Store interface;
	// a direct gorm count verifies the dedup behaviour.
	db := testutil.NewTestDB(t)
	var count int64
	if err := db.Model(&store.Link{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	// The previous NewTestDB gave us a fresh DB; but the
	// Store used a *different* DB instance from the first
	// testutil.NewTestDB call. So we cannot assert here on
	// that DB. Instead, create a parallel store and assert.
	_ = db

	// Re-test with one DB to get a meaningful assertion.
	db = testutil.NewTestDB(t)
	st := sqlite.New(db)
	link := store.Link{Hash: store.HashOf("u"), Domain: "x.com", URL: "https://x.com/u"}
	if err := st.CreateLinksInBatches(context.Background(), []store.Link{link, link}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var c2 int64
	if err := db.Model(&store.Link{}).Count(&c2).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if c2 != 1 {
		t.Fatalf("dedup: want 1 row, got %d", c2)
	}
}
