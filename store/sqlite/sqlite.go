// Package sqlite is the default gorm-backed SQLite Store.
package sqlite

import (
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Open opens (or creates) a SQLite database at path.
func Open(path string) (*gorm.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("sqlite: path is required")
	}
	return gorm.Open(sqlite.Open(path), &gorm.Config{})
}
