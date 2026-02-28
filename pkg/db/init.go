package db

import (
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func InitialiseDatabase() (*gorm.DB, error) {
	dirpath := filepath.Join(".", "data")
	err := os.Mkdir(dirpath, os.ModePerm)
	if err != nil && err == os.ErrExist {
		return nil, fmt.Errorf("failed to create directory '%s': %s", dirpath, err.Error())
	}

	db, err := gorm.Open(sqlite.Open("data/test.db"), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %s", err.Error())
	}

	// ctx := context.Background()

	db.AutoMigrate(&Device{}, &User{}, &AuthSession{}, &Question{}, &Session{})
	return db, nil
}
