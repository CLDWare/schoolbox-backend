package main

import (
	"os"

	"github.com/CLDWare/schoolbox-backend/config"
	"github.com/CLDWare/schoolbox-backend/internal/janitor"
	models "github.com/CLDWare/schoolbox-backend/pkg/db"
	"github.com/CLDWare/schoolbox-backend/pkg/logger"
	"github.com/joho/godotenv"
)

func main() {

	// Initialize logger with the updated configuration
	logger.Init()

	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		logger.Info(".env file not found, proceeding with environment variables")
	}

	// Force reload configuration after .env is loaded
	config.ForceReload()

	// Load configuration
	cfg := config.Get()

	// Initialise Database
	db, err := models.InitialiseDatabase()
	if err != nil {
		logger.Err(err)
		os.Exit(1)
	}

	jan := janitor.NewJanitor(cfg, db, false)

	jan.RunShort()
}
