package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CLDWare/schoolbox-backend/api"
	"github.com/CLDWare/schoolbox-backend/config"
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

	// Create API instance
	apiInstance := api.NewAPI()

	// Create mux with routes
	mux := apiInstance.CreateMux()

	// Apply middleware
	handler := api.ApplyMiddleware(mux)

	// Server configuration
	server := &http.Server{
		Addr:         cfg.GetServerAddress(),
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting server on", server.Addr)
		logger.Info("Environment:", cfg.App.Environment)
		logger.Info("Debug mode:", cfg.App.Debug)
		logger.Info("Application:", cfg.App.Name, "v"+cfg.App.Version)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Err("Server failed to start:", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		logger.Err("Server forced to shutdown:", err)
		os.Exit(1)
	}

	logger.Info("Server exited")
}
