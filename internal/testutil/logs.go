package testutil

import (
	"bytes"
	"sync"
	"testing"

	"github.com/rs/zerolog"
)

// SafeBuffer is a goroutine-safe wrapper around bytes.Buffer.
// zerolog is concurrent-safe but tests that snapshot the buffer
// need a stable view, so this type provides a mutex on every
// read and write.
type SafeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write implements io.Writer.
func (b *SafeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// String returns a snapshot of the buffer contents.
func (b *SafeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// Len returns the buffer's length.
func (b *SafeBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

// Bytes returns a copy of the buffer contents.
func (b *SafeBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]byte, b.buf.Len())
	copy(out, b.buf.Bytes())
	return out
}

// NewTestLogger returns a zerolog.Logger that writes to a
// SafeBuffer (returned alongside it). The logger is set to
// debug level so test code can verify debug-level messages.
func NewTestLogger(t *testing.T) (*zerolog.Logger, *SafeBuffer) {
	t.Helper()
	buf := &SafeBuffer{}
	lg := zerolog.New(buf).Level(zerolog.DebugLevel)
	return &lg, buf
}
