package testutil

import (
	"net/http"
	"sync"
	"testing"
)

func TestSafeBuffer_ConcurrentWrite(t *testing.T) {
	b := &SafeBuffer{}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = b.Write([]byte("x"))
		}()
	}
	wg.Wait()
	if got := b.Len(); got != 50 {
		t.Fatalf("len: want 50, got %d", got)
	}
}

func TestNewTestLogger_WritesToBuffer(t *testing.T) {
	lg, buf := NewTestLogger(t)
	lg.Info().Str("k", "v").Msg("hello")
	if got := buf.String(); got == "" {
		t.Fatal("expected non-empty log")
	}
}

func TestNewTestDB_OpensAndMigrates(t *testing.T) {
	db := NewTestDB(t)
	if db == nil {
		t.Fatal("nil db")
	}
	// Smoke test: sql.DB is open and responsive.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("DB(): %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	// Print actual table names so we can pick the right ones
	// in production code.
	rows, err := sqlDB.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		names = append(names, n)
	}
	t.Logf("tables: %v", names)
}

func TestNewTestServer_GetsAURL(t *testing.T) {
	srv := NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	if srv.URL == "" {
		t.Fatal("server URL is empty")
	}
}
