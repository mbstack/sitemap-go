package storage

import (
	"log"
	"path/filepath"

	"github.com/MegaBytee/sitemap-go/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func NewSqlite(name string) *gorm.DB {
	dataDir, err := config.GetDataDir()
	if err != nil {
		log.Fatalf("Failed to get project main directory: %v", err)
		return nil
	}
	sqlite_db := filepath.Join(dataDir, name)

	db, err := gorm.Open(sqlite.Open(sqlite_db), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database")
		return nil
	}
	return db

}

// whereIn : "id IN ?"
func updateOneFieldInBatches(db *gorm.DB, model interface{}, whereIn string, ids []string, key string, value any, batchSize int) error {

	// Start a transaction
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for i := 0; i < len(ids); i += batchSize {
		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}

		// Update the batch
		if err := tx.Model(model).Where(whereIn, ids[i:end]).Update(key, value).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// Commit the transaction
	return tx.Commit().Error

}
