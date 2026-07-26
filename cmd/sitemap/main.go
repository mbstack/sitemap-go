// Command sitemap is the mbstack/sitemap-go CLI.
//
// Usage:
//
//	sitemap scan <url> --data <dir> [--concurrency N]
//	                   [--min-delay 10ms] [--max-delay 500ms]
//	                   [--log-level info] [--pretty]
//
// Exit codes:
//
//	0  success
//	1  invalid flags / config
//	2  fetch / parse / store error
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mbstack/sitemap-go"
	"github.com/mbstack/sitemap-go/config"
	"github.com/mbstack/sitemap-go/logger"
)

func main() {
	code, err := run(os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sitemap:", err)
	}
	os.Exit(code)
}

func run(args []string, stdout, stderr *os.File) (int, error) {
	fs := flag.NewFlagSet("sitemap", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		dataDir     string
		concurrency int
		minDelay    time.Duration
		maxDelay    time.Duration
		level       string
		pretty      bool
		limit       int
		showHelp    bool
	)
	fs.StringVar(&dataDir, "data", "./data", "directory for the SQLite database and Colly cache")
	fs.IntVar(&concurrency, "concurrency", 16, "worker pool size for ScanSitemapIndex")
	fs.DurationVar(&minDelay, "min-delay", 10*time.Millisecond, "minimum delay between requests")
	fs.DurationVar(&maxDelay, "max-delay", 500*time.Millisecond, "maximum delay between requests")
	fs.StringVar(&level, "log-level", "info", "log level: trace, debug, info, warn, error")
	fs.BoolVar(&pretty, "pretty", false, "human-readable log output (auto-detected on TTY)")
	fs.IntVar(&limit, "limit", 100, "max sitemaps to scan per call")
	fs.BoolVar(&showHelp, "h", false, "show help")

	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: sitemap scan <url> [flags]")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "Flags:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1, err
	}
	if showHelp || fs.NArg() < 1 {
		fs.Usage()
		return 0, nil
	}
	cmd := fs.Arg(0)
	if cmd != "scan" {
		fs.Usage()
		return 1, fmt.Errorf("unknown subcommand %q (only 'scan' is supported)", cmd)
	}
	if fs.NArg() < 2 {
		fs.Usage()
		return 1, fmt.Errorf("scan requires a site URL")
	}
	siteURL := fs.Arg(1)

	lg := logger.New(logger.Options{Level: level, Pretty: pretty})

	cfg := config.DefaultConfig()
	cfg.DataDir = dataDir
	cfg.Logger = lg
	cfg.Concurrency = concurrency
	cfg.MinDelay = minDelay
	cfg.MaxDelay = maxDelay
	if err := cfg.Validate(); err != nil {
		return 1, err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	s, err := sitemap.NewScanner(ctx, cfg)
	if err != nil {
		return 1, err
	}
	defer s.Close()

	if err := s.ScanSite(ctx, siteURL); err != nil {
		lg.Error().Err(err).Msg("ScanSite failed")
		return 2, err
	}
	if err := s.ScanSitemapIndex(ctx, limit); err != nil {
		lg.Error().Err(err).Msg("ScanSitemapIndex failed")
		return 2, err
	}
	lg.Info().Str("site", siteURL).Msg("done")
	return 0, nil
}
