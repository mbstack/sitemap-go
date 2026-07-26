package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestNew_Defaults(t *testing.T) {
	lg := New(Options{})
	if lg == nil {
		t.Fatal("expected non-nil logger")
	}
	// Out defaults to stderr — we can't easily assert on stderr;
	// instead, capture by passing a buffer and verify the level
	// fallback path also writes to a working logger.
	if got := lg.GetLevel(); got != zerolog.InfoLevel {
		t.Fatalf("default level: want info, got %s", got)
	}
}

func TestNew_LevelFallback(t *testing.T) {
	cases := []struct {
		in   string
		want zerolog.Level
	}{
		{"", zerolog.InfoLevel},
		{"unknown-level", zerolog.InfoLevel},
		{"DEBUG", zerolog.DebugLevel},
		{"warn", zerolog.WarnLevel},
		{"error", zerolog.ErrorLevel},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			lg := New(Options{Level: tc.in})
			if lg.GetLevel() != tc.want {
				t.Fatalf("level(%q) = %s, want %s", tc.in, lg.GetLevel(), tc.want)
			}
		})
	}
}

func TestNew_WritesToOut(t *testing.T) {
	buf := &bytes.Buffer{}
	lg := New(Options{Out: buf})
	lg.Info().Str("k", "v").Msg("hello")

	out := buf.String()
	if !strings.Contains(out, "hello") {
		t.Fatalf("output missing msg: %q", out)
	}
	if !strings.Contains(out, `"k":"v"`) {
		t.Fatalf("output missing field: %q", out)
	}
}

func TestNew_PrettyIsValidJSON_False(t *testing.T) {
	buf := &bytes.Buffer{}
	lg := New(Options{Out: buf})
	lg.Info().Msg("json-line")

	// Default (non-pretty) must emit a single JSON line.
	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m); err != nil {
		t.Fatalf("non-pretty output is not valid JSON: %v\n%s", err, buf.String())
	}
	if m["message"] != "json-line" {
		t.Fatalf("expected message field, got %v", m["message"])
	}
}

func TestNew_PrettyIsHumanReadable(t *testing.T) {
	buf := &bytes.Buffer{}
	lg := New(Options{Out: buf, Pretty: true})
	lg.Info().Msg("pretty-line")

	out := buf.String()
	if !strings.Contains(out, "pretty-line") {
		t.Fatalf("pretty output missing msg: %q", out)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("pretty output should not start with '{': %q", out)
	}
}
