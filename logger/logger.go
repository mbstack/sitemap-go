// Package logger configures a zerolog.Logger for the sitemap-go
// library. See New for the default wiring and Options for tunables.
package logger

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Options configures the logger returned by New.
//
// All fields are optional; sensible defaults are filled in by New.
type Options struct {
	// Level is one of "trace", "debug", "info", "warn", "error",
	// "fatal", "panic". Empty or unrecognised values fall back to
	// "info". Comparison is case-insensitive.
	Level string

	// Pretty switches the writer to a human-readable console
	// format. Recommended for interactive CLI use.
	Pretty bool

	// Out is the destination writer. Defaults to os.Stderr when
	// nil. Tests typically pass a *bytes.Buffer.
	Out io.Writer
}

// New returns a configured zerolog.Logger. The returned logger
// always carries a "ts" field formatted as RFC3339.
//
// Example:
//
//	lg := logger.New(logger.Options{Level: "debug", Pretty: true})
//	lg.Info().Str("url", "https://example.com").Msg("scanning site")
func New(opts Options) *zerolog.Logger {
	if opts.Out == nil {
		opts.Out = os.Stderr
	}

	lvl, err := zerolog.ParseLevel(strings.ToLower(opts.Level))
	if err != nil || opts.Level == "" {
		lvl = zerolog.InfoLevel
	}
	// We deliberately do NOT call zerolog.SetGlobalLevel here.
	// Global state would leak across packages and tests. Set the
	// level on the returned logger instead.
	zerolog.TimeFieldFormat = time.RFC3339

	var lg zerolog.Logger
	if opts.Pretty {
		lg = zerolog.New(zerolog.ConsoleWriter{Out: opts.Out}).
			With().Timestamp().Logger()
	} else {
		lg = zerolog.New(opts.Out).With().Timestamp().Logger()
	}
	lg = lg.Level(lvl)
	return &lg
}
