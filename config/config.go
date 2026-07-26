// Package config defines Config, the validated runtime configuration
// for a Scanner. See DefaultConfig for sensible zero values and
// Config.Validate for the rules.
package config

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mbstack/sitemap-go/internal/sitemaperr"
	"github.com/rs/zerolog"
)

// Config is the runtime configuration for a Scanner. Construct with
// DefaultConfig, set the required fields, and pass the result to
// NewScanner. Validate is called internally by NewScanner.
type Config struct {
	// DataDir is the directory for the SQLite database and the
	// Colly cache. Required. The directory is created by
	// EnsureDataDir if it does not already exist.
	DataDir string

	// Logger receives all library log output. Required. The
	// library never writes to a default logger; if you do not
	// supply one, the constructor returns an error wrapping
	// sitemap.ErrInvalidConfig.
	Logger *zerolog.Logger

	// HTTPClient is used for robots.txt and sitemap fetches.
	// If nil, internal/httpx.New is used with the fields below.
	HTTPClient *http.Client

	// Concurrency is the worker pool size for ScanSitemapIndex.
	// Default 16. Must be > 0.
	Concurrency int

	// MinDelay and MaxDelay bound the random sleep between
	// requests. Defaults 10ms and 500ms.
	MinDelay time.Duration
	MaxDelay time.Duration

	// UserAgent is sent on every request. Default
	// "mbstack-sitemap-go/1.0 (+https://mbstack.dev)".
	UserAgent string

	// ProxyURL, if non-empty, is set as http.Transport.Proxy on
	// the default HTTPClient. Ignored when HTTPClient is non-nil.
	ProxyURL string
}

// DefaultConfig returns a Config with sensible zero-value defaults
// filled in. DataDir and Logger are still required and must be set
// by the caller.
func DefaultConfig() Config {
	return Config{
		Concurrency: 16,
		MinDelay:    10 * time.Millisecond,
		MaxDelay:    500 * time.Millisecond,
		UserAgent:   "mbstack-sitemap-go/1.0 (+https://mbstack.dev)",
	}
}

// Validate applies defaults and checks invariants. It mutates c
// in place: missing zero values are filled from DefaultConfig().
// Returns an error wrapping sitemaperr.ErrInvalidConfig when an
// invariant fails.
func (c *Config) Validate() error {
	if c.DataDir == "" {
		return fmt.Errorf("%w: DataDir is required", sitemaperr.ErrInvalidConfig)
	}
	if c.Logger == nil {
		return fmt.Errorf("%w: Logger is required", sitemaperr.ErrInvalidConfig)
	}
	if c.Concurrency <= 0 {
		c.Concurrency = 16
	}
	if c.MinDelay <= 0 {
		c.MinDelay = 10 * time.Millisecond
	}
	if c.MaxDelay <= 0 {
		c.MaxDelay = 500 * time.Millisecond
	}
	if c.UserAgent == "" {
		c.UserAgent = "mbstack-sitemap-go/1.0"
	}
	if c.MaxDelay < c.MinDelay {
		return fmt.Errorf("%w: MaxDelay (%s) < MinDelay (%s)",
			sitemaperr.ErrInvalidConfig, c.MaxDelay, c.MinDelay)
	}
	return nil
}

// EnsureDataDir creates c.DataDir (and any parents) if it does
// not already exist. Idempotent. Returns os.MkdirAll's error on
// filesystem failure.
func (c *Config) EnsureDataDir() error {
	return os.MkdirAll(c.DataDir, 0o755)
}

// CollyDir is the directory Colly uses for cache and SQLite-backed
// HTTP storage. It is always <DataDir>/colly.
func (c *Config) CollyDir() string {
	return filepath.Join(c.DataDir, "colly")
}

// DBPath is the SQLite database file path. It is always
// <DataDir>/sitemap.db.
func (c *Config) DBPath() string {
	return filepath.Join(c.DataDir, "sitemap.db")
}
