// Package sitemap: Fetcher interface and Colly-backed default.
package sitemap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/mbstack/sitemap-go/config"
	"github.com/mbstack/sitemap-go/internal/httpx"
	"github.com/mbstack/sitemap-go/internal/sitemaperr"
	"github.com/rs/zerolog"
)

// Fetcher abstracts a single HTTP GET. The Scanner uses it to
// fetch robots.txt and sitemap documents. The default
// implementation is CollyFetcher, which is built on
// gocolly/colly and supports the cache + storage already wired
// in Config.DataDir.
type Fetcher interface {
	// Get returns the body and the response status. A non-2xx
	// status is not an error from the protocol's point of view
	// but the caller usually wants to treat it as one; see
	// GetOK.
	Get(ctx context.Context, u string) (body []byte, status int, err error)
}

// GetOK is a convenience that returns ErrFetch (wrapped) on
// any non-2xx status. Implementations of Fetcher do not have
// to provide this; it is a free function so callers can use
// it without depending on a specific implementation.
func GetOK(ctx context.Context, f Fetcher, u string) ([]byte, error) {
	body, status, err := f.Get(ctx, u)
	if err != nil {
		return nil, sitemaperr.Wrap("fetch", err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: %s: status %d", sitemaperr.ErrFetch, u, status)
	}
	return body, nil
}

// fetchRobots performs a single direct HTTP GET against robotsURL
// using a fresh *http.Client. It deliberately bypasses the colly
// Fetcher used for sitemap bodies, so robots.txt fetches are not
// subject to colly's HTTP/2 stack or cache.
//
// The caller is expected to be GetSitemapURLs; this function is
// not part of the package's public surface.
func fetchRobots(ctx context.Context, robotsURL, userAgent string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, nil)
	if err != nil {
		return nil, sitemaperr.Wrap("robots: new request", err)
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, sitemaperr.Wrap("robots: fetch", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: %s: status %d", sitemaperr.ErrFetch, robotsURL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, sitemaperr.Wrap("robots: read body", err)
	}
	return body, nil
}

// CollyFetcher is the default Fetcher implementation, backed by
// a fresh *colly.Collector per call. A fresh collector per call
// avoids the audit-flagged "callbacks accumulate across
// iterations" footgun.
type CollyFetcher struct {
	cfg  config.Config
	log  *zerolog.Logger
	base *http.Client
}

// NewCollyFetcher returns a CollyFetcher built from cfg. cfg
// must have been Validate'd; Logger and DataDir are required.
// The returned fetcher owns no goroutines and is safe for
// concurrent use.
func NewCollyFetcher(cfg config.Config, log *zerolog.Logger) (*CollyFetcher, error) {
	if log == nil {
		return nil, fmt.Errorf("%w: logger is required", sitemaperr.ErrInvalidConfig)
	}
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("%w: DataDir is required", sitemaperr.ErrInvalidConfig)
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = httpx.New(httpx.Options{
			Timeout:   30 * time.Second,
			ProxyURL:  cfg.ProxyURL,
			UserAgent: cfg.UserAgent,
		})
	}
	return &CollyFetcher{cfg: cfg, log: log, base: httpClient}, nil
}

// Get fetches u using a fresh colly.Collector and returns the
// body bytes plus the HTTP status code.
func (f *CollyFetcher) Get(ctx context.Context, u string) ([]byte, int, error) {
	if u == "" {
		return nil, 0, fmt.Errorf("%w: empty url", sitemaperr.ErrFetch)
	}
	parsed, err := url.Parse(u)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, 0, fmt.Errorf("%w: invalid url %q", sitemaperr.ErrFetch, u)
	}

	c := colly.NewCollector(
		colly.Async(false),
	)
	c.SetClient(f.base)

	type result struct {
		body   []byte
		status int
		err    error
	}
	ch := make(chan result, 1)

	c.OnResponse(func(r *colly.Response) {
		// copy bytes: colly reuses the underlying buffer
		buf := make([]byte, len(r.Body))
		copy(buf, r.Body)
		ch <- result{body: buf, status: r.StatusCode}
	})
	c.OnError(func(r *colly.Response, cerr error) {
		status := 0
		if r != nil {
			status = r.StatusCode
		}
		ch <- result{status: status, err: cerr}
	})

	if err := c.Visit(u); err != nil {
		// Domain resolution / parse errors land here.
		if isCancelled(ctx) {
			return nil, 0, ctx.Err()
		}
		return nil, 0, fmt.Errorf("%w: %s: %v", sitemaperr.ErrFetch, u, err)
	}

	select {
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			if isCancelled(ctx) {
				return nil, r.status, ctx.Err()
			}
			return nil, r.status, fmt.Errorf("%w: %s: %v", sitemaperr.ErrFetch, u, r.err)
		}
		return r.body, r.status, nil
	}
}

// isCancelled reports whether ctx has been cancelled.
func isCancelled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// domainOf returns the host part of u, or "" if unparseable.
func domainOf(u string) string {
	parsed, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

// ioCopy is a tiny helper used by tests to turn an io.Reader
// into []byte. It's here (not in internal/) so test code in
// the root package can use it without exporting more API.
func ioCopy(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, errors.New("nil reader")
	}
	var buf strings.Builder
	tmp := make([]byte, 1024)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return []byte(buf.String()), nil
			}
			return nil, err
		}
	}
}
