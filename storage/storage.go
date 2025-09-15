package storage

import (
	"log"
	"sync"

	"gorm.io/gorm"
)

type Storage struct {
	Data *gorm.DB
	mu   sync.RWMutex
}

func New() *Storage {

	return &Storage{
		Data: NewSqlite("storage.db"),
	}
}

func (s *Storage) Close() {
	sqlDB, err := s.Data.DB()
	if err != nil {
		log.Fatal(err)
	}
	sqlDB.Close() // Close the connection

}
func (s *Storage) Config() *Storage {

	if s.Data == nil {
		return nil
	}

	err := s.Data.AutoMigrate(tables...)
	if err != nil {
		return nil
	}

	return s
}
