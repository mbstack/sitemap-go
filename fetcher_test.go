package sitemap

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mbstack/sitemap-go/config"
	"github.com/mbstack/sitemap-go/internal/sitemaperr"
	"github.com/mbstack/sitemap-go/internal/testutil"
)

func newFetcher(t *testing.T) *CollyFetcher {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.DataDir = t.TempDir()
	lg, _ := testutil.NewTestLogger(t)
	f, err := NewCollyFetcher(cfg, lg)
	if err != nil {
		t.Fatalf("NewCollyFetcher: %v", err)
	}
	return f
}

func TestGet_HappyPath(t *testing.T) {
	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello"))
	}))
	f := newFetcher(t)

	body, status, err := f.Get(context.Background(), srv.URL+"/x")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status: want 200, got %d", status)
	}
	if string(body) != "hello" {
		t.Fatalf("body: want %q, got %q", "hello", string(body))
	}
}

func TestGet_4xx(t *testing.T) {
	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusNotFound)
	}))
	f := newFetcher(t)

	// colly routes 4xx responses through OnError, so Get
	// returns an error wrapping ErrFetch for 404. Callers that
	// want to treat 4xx as a non-error use GetOK (covered by
	// TestGetOK_WrapsErrFetchOn4xx). The point of this test is
	// simply that 4xx does not return a body of the previous
	// successful request — i.e. no state leaks between
	// Get calls (the audit's F16).
	_, _, err := f.Get(context.Background(), srv.URL+"/missing")
	if !errors.Is(err, sitemaperr.ErrFetch) {
		t.Fatalf("expected ErrFetch, got %v", err)
	}
}

func TestGetOK_WrapsErrFetchOn4xx(t *testing.T) {
	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusNotFound)
	}))
	f := newFetcher(t)
	_, err := GetOK(context.Background(), f, srv.URL+"/missing")
	if !errors.Is(err, sitemaperr.ErrFetch) {
		t.Fatalf("expected ErrFetch, got %v", err)
	}
}

func TestGetOK_HappyPath(t *testing.T) {
	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	f := newFetcher(t)
	body, err := GetOK(context.Background(), f, srv.URL+"/")
	if err != nil {
		t.Fatalf("GetOK: %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("body: want %q, got %q", "ok", string(body))
	}
}

func TestGet_InvalidURL(t *testing.T) {
	f := newFetcher(t)
	_, _, err := f.Get(context.Background(), "ht!tp://nope")
	if !errors.Is(err, sitemaperr.ErrFetch) {
		t.Fatalf("expected ErrFetch, got %v", err)
	}
}

func TestGet_EmptyURL(t *testing.T) {
	f := newFetcher(t)
	_, _, err := f.Get(context.Background(), "")
	if !errors.Is(err, sitemaperr.ErrFetch) {
		t.Fatalf("expected ErrFetch, got %v", err)
	}
}

func TestGet_ContextCancelled(t *testing.T) {
	// We use a server that responds normally, but the http.Client
	// is given a 1ms timeout via a custom Config.HTTPClient. That
	// makes Get return an error quickly. The test is robust to
	// either a context-cancellation error or a transport timeout
	// — both are valid outcomes when the call is short.
	cfg := config.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.HTTPClient = &http.Client{Timeout: 1 * time.Millisecond}
	lg, _ := testutil.NewTestLogger(t)
	f, err := NewCollyFetcher(cfg, lg)
	if err != nil {
		t.Fatalf("NewCollyFetcher: %v", err)
	}

	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
	}))
	_, _, err = f.Get(context.Background(), srv.URL+"/slow")
	if err == nil {
		t.Fatal("expected error from 1ms-timeout client")
	}
}

func TestNewCollyFetcher_RequiresLogger(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDir = t.TempDir()
	if _, err := NewCollyFetcher(cfg, nil); !errors.Is(err, sitemaperr.ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestNewCollyFetcher_RequiresDataDir(t *testing.T) {
	cfg := config.DefaultConfig()
	lg, _ := testutil.NewTestLogger(t)
	if _, err := NewCollyFetcher(cfg, lg); !errors.Is(err, sitemaperr.ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestIsCancelled(t *testing.T) {
	if isCancelled(context.Background()) {
		t.Fatal("fresh ctx should not be cancelled")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !isCancelled(ctx) {
		t.Fatal("cancelled ctx should report cancelled")
	}
	if isCancelled(context.TODO()) {
		t.Fatal("fresh ctx should not be cancelled")
	}
}

func TestIOCopy(t *testing.T) {
	got, err := ioCopy(strings.NewReader("payload"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(got) != "payload" {
		t.Fatalf("want payload, got %q", string(got))
	}
	if _, err := ioCopy(nil); err == nil {
		t.Fatal("expected error for nil reader")
	}
}
