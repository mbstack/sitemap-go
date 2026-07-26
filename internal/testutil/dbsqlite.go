package testutil

import (
	"path/filepath"
	"testing"

	"github.com/mbstack/sitemap-go/store/sqlite"
	"gorm.io/gorm"
)

// NewTestDB opens a fresh SQLite database inside the test's
// temp directory, runs the store migrations, and registers a
// t.Cleanup that closes the underlying sql.DB. The temp
// directory is removed by t.TempDir's own cleanup.
//
// We use a real file (not ":memory:") because the gorm SQLite
// driver uses connection pooling, and ":memory:" databases are
// private to each connection. Real files also support WAL
// mode, which concurrent tests rely on.
func NewTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("open sqlite at %s: %v", path, err)
	}
	if err := sqlite.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}
