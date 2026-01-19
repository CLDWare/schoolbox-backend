package db

import (
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func InitialiseDatabase() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("data/test.db"), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %s", err.Error())
	}

	// ctx := context.Background()

	db.AutoMigrate(&Device{}, &User{}, &AuthSession{}, &Question{}, &Session{})
	return db, nil
}
