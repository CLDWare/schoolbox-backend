package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestGet_Singleton(t *testing.T) {
	// Reset for clean test
	Reload()

	// Get config twice
	cfg1 := Get()
	cfg2 := Get()

	// Should be the same instance
	if cfg1 != cfg2 {
		t.Error("Get() should return the same instance (singleton pattern)")
	}
}

func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		shouldPanic bool
	}{
		{
			name: "valid config",
			env: map[string]string{
				"SERVER_PORT": "8080",
				"ENV":         "development",
				"LOG_LEVEL":   "info",
			},
			shouldPanic: false,
		},
		{
			name: "invalid port",
			env: map[string]string{
				"SERVER_PORT": "invalid",
			},
			shouldPanic: true,
		},
		{
			name: "invalid environment",
			env: map[string]string{
				"ENV": "invalid",
			},
			shouldPanic: true,
		},
		{
			name: "invalid log level",
			env: map[string]string{
				"LOG_LEVEL": "invalid",
			},
			shouldPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Backup original environment
			originalEnv := backupEnv()
			defer restoreEnv(originalEnv)

			// Set test environment
			for key, value := range tt.env {
				os.Setenv(key, value)
			}

			// Reset singleton
			Reload()

			// Test
			if tt.shouldPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Error("Expected panic but didn't get one")
					}
				}()
				Get()
			} else {
				cfg := Get()
				if cfg == nil {
					t.Error("Expected valid config but got nil")
				}
			}
		})
	}
}

func TestConfig_Methods(t *testing.T) {
	// Backup original environment
	originalEnv := backupEnv()
	defer restoreEnv(originalEnv)

	// Set test environment
	os.Setenv("ENV", "development")
	os.Setenv("SERVER_HOST", "127.0.0.1")
	os.Setenv("SERVER_PORT", "9000")

	// Reset singleton
	Reload()

	cfg := Get()

	// Test IsDevelopment
	if !cfg.IsDevelopment() {
		t.Error("Expected IsDevelopment() to return true")
	}

	// Test IsProduction
	if cfg.IsProduction() {
		t.Error("Expected IsProduction() to return false")
	}

	// Test GetServerAddress
	expectedAddr := "127.0.0.1:9000"
	if addr := cfg.GetServerAddress(); addr != expectedAddr {
		t.Errorf("Expected server address %s, got %s", expectedAddr, addr)
	}
}

func TestConfig_EnvironmentVariables(t *testing.T) {
	// Backup original environment
	originalEnv := backupEnv()
	defer restoreEnv(originalEnv)

	// Set specific test values
	testValues := map[string]string{
		"SERVER_HOST":         "test-host",
		"SERVER_PORT":         "9999",
		"SERVER_READ_TIMEOUT": "30s",
		"APP_NAME":            "test-app",
		"APP_VERSION":         "2.0.0",
		"DEBUG":               "true",
		"DB_HOST":             "db-host",
		"DB_PORT":             "5433",
		"LOG_LEVEL":           "debug",
	}

	for key, value := range testValues {
		os.Setenv(key, value)
	}

	// Reset singleton
	Reload()

	cfg := Get()

	// Test server config
	if cfg.Server.Host != "test-host" {
		t.Errorf("Expected server host test-host, got %s", cfg.Server.Host)
	}

	if cfg.Server.Port != "9999" {
		t.Errorf("Expected server port 9999, got %s", cfg.Server.Port)
	}

	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("Expected read timeout 30s, got %v", cfg.Server.ReadTimeout)
	}

	// Test app config
	if cfg.App.Name != "test-app" {
		t.Errorf("Expected app name test-app, got %s", cfg.App.Name)
	}

	if cfg.App.Version != "2.0.0" {
		t.Errorf("Expected app version 2.0.0, got %s", cfg.App.Version)
	}

	if !cfg.App.Debug {
		t.Error("Expected debug to be true")
	}
}

// Helper functions for testing
func backupEnv() map[string]string {
	env := make(map[string]string)
	for _, pair := range os.Environ() {
		if idx := strings.Index(pair, "="); idx != -1 {
			key := pair[:idx]
			value := pair[idx+1:]
			env[key] = value
		}
	}
	return env
}

func restoreEnv(env map[string]string) {
	os.Clearenv()
	for key, value := range env {
		os.Setenv(key, value)
	}
}
