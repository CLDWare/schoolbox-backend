package config

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config holds application configuration
type Config struct {
	// Server configuration
	Server ServerConfig `json:"server"`

	// Logging configuration
	Logging LoggingConfig `json:"logging"`

	// Application configuration
	App AppConfig `json:"app"`
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Host         string        `json:"host"`
	Port         string        `json:"port"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	IdleTimeout  time.Duration `json:"idle_timeout"`
}

// LoggingConfig holds logging-specific configuration
type LoggingConfig struct {
	Level string `json:"level"`
}

// AppConfig holds application-specific configuration
type AppConfig struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Environment string `json:"environment"`
	Debug       bool   `json:"debug"`
}

var (
	instance *Config
	once     sync.Once
	mu       sync.RWMutex
)

// Get returns the singleton configuration instance
func Get() *Config {
	mu.RLock()
	if instance != nil {
		defer mu.RUnlock()
		return instance
	}
	mu.RUnlock()

	once.Do(func() {
		mu.Lock()
		defer mu.Unlock()
		instance = loadConfig()
	})
	return instance
}

// Load loads configuration from environment variables (deprecated, use Get() instead)
func Load() *Config {
	return Get()
}

// load loads configuration from environment variables
func loadConfig() *Config {
	cfg := &Config{
		Server: ServerConfig{
			Host:         getEnv("SERVER_HOST", "localhost"),
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  getEnvAsDuration("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getEnvAsDuration("SERVER_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getEnvAsDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
		},
		Logging: LoggingConfig{
			Level: getEnv("LOG_LEVEL", "info"),
		},
		App: AppConfig{
			Name:        getEnv("APP_NAME", "schoolbox-backend"),
			Version:     getEnv("APP_VERSION", "1.0.0"),
			Environment: getEnv("ENV", "development"),
			Debug:       getEnvAsBool("DEBUG", false),
		},
	}

	// Validate configuration
	if err := cfg.validate(); err != nil {
		panic(fmt.Sprintf("Invalid configuration: %v", err))
	}

	return cfg
}

// validate validates the configuration
func (c *Config) validate() error {
	// Validate server port
	if port, err := strconv.Atoi(c.Server.Port); err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid server port: %s", c.Server.Port)
	}

	// Validate environment
	validEnvs := []string{"development", "staging", "production"}
	if !contains(validEnvs, c.App.Environment) {
		return fmt.Errorf("invalid environment: %s (must be one of: %s)",
			c.App.Environment, strings.Join(validEnvs, ", "))
	}

	// Validate log level
	validLevels := []string{"info", "warn", "error"}
	if !slices.Contains(validLevels, strings.ToLower(c.Logging.Level)) {
		return fmt.Errorf("invalid log level: %s (must be one of: %s)",
			c.Logging.Level, strings.Join(validLevels, ", "))
	}

	return nil
}

// IsDevelopment returns true if the app is running in development mode
func (c *Config) IsDevelopment() bool {
	return c.App.Environment == "development"
}

// IsProduction returns true if the app is running in production mode
func (c *Config) IsProduction() bool {
	return c.App.Environment == "production"
}

// GetServerAddress returns the server address in the format "host:port"
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%s", c.Server.Host, c.Server.Port)
}

// Reload reloads the configuration (useful for testing or after loading .env files)
func Reload() {
	mu.Lock()
	defer mu.Unlock()
	once = sync.Once{}
	instance = nil
}

// ForceReload forces an immediate reload of the configuration
func ForceReload() {
	mu.Lock()
	defer mu.Unlock()
	instance = loadConfig()
}

// getEnv gets an environment variable with a fallback value
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// getEnvAsBool gets an environment variable as boolean with a fallback value
func getEnvAsBool(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return fallback
}

// getEnvAsDuration gets an environment variable as duration with a fallback value
func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return fallback
}

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
