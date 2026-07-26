package config

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mbstack/sitemap-go/internal/sitemaperr"
	"github.com/rs/zerolog"
)

func newLogger() *zerolog.Logger {
	lg := zerolog.New(nil).Level(zerolog.Disabled)
	return &lg
}

func TestDefaultConfig_ZeroFieldsAreSensible(t *testing.T) {
	c := DefaultConfig()
	if c.Concurrency != 16 {
		t.Errorf("Concurrency: want 16, got %d", c.Concurrency)
	}
	if c.MinDelay != 10*time.Millisecond {
		t.Errorf("MinDelay: want 10ms, got %s", c.MinDelay)
	}
	if c.MaxDelay != 500*time.Millisecond {
		t.Errorf("MaxDelay: want 500ms, got %s", c.MaxDelay)
	}
	if c.UserAgent == "" {
		t.Error("UserAgent should have a default")
	}
}

func TestValidate_MissingDataDir(t *testing.T) {
	c := DefaultConfig()
	c.Logger = newLogger()
	if err := c.Validate(); !errors.Is(err, sitemaperr.ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestValidate_MissingLogger(t *testing.T) {
	c := DefaultConfig()
	c.DataDir = t.TempDir()
	if err := c.Validate(); !errors.Is(err, sitemaperr.ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestValidate_FillsZeroDefaults(t *testing.T) {
	c := Config{DataDir: t.TempDir(), Logger: newLogger()}
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Concurrency != 16 || c.MinDelay != 10*time.Millisecond ||
		c.MaxDelay != 500*time.Millisecond || c.UserAgent == "" {
		t.Errorf("Validate did not fill zero defaults: %+v", c)
	}
}

func TestValidate_MaxLessThanMin(t *testing.T) {
	c := DefaultConfig()
	c.DataDir = t.TempDir()
	c.Logger = newLogger()
	c.MinDelay = 500 * time.Millisecond
	c.MaxDelay = 10 * time.Millisecond
	if err := c.Validate(); !errors.Is(err, sitemaperr.ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestValidate_AllFieldsOK(t *testing.T) {
	c := DefaultConfig()
	c.DataDir = t.TempDir()
	c.Logger = newLogger()
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureDataDir_CreatesAndIsIdempotent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")
	c := Config{DataDir: dir}

	if err := c.EnsureDataDir(); err != nil {
		t.Fatalf("first EnsureDataDir: %v", err)
	}
	// second call must not error
	if err := c.EnsureDataDir(); err != nil {
		t.Fatalf("second EnsureDataDir should be idempotent: %v", err)
	}
}

func TestPathHelpers(t *testing.T) {
	c := Config{DataDir: "/x/data"}
	if got, want := c.CollyDir(), filepath.Join("/x/data", "colly"); got != want {
		t.Errorf("CollyDir: want %s, got %s", want, got)
	}
	if got, want := c.DBPath(), filepath.Join("/x/data", "sitemap.db"); got != want {
		t.Errorf("DBPath: want %s, got %s", want, got)
	}
}
