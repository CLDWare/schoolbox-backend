package logger

import (
	"log"
	"os"
	"strings"

	"github.com/CLDWare/schoolbox-backend/config"
)

var (
	InfoLogger    *log.Logger
	WarningLogger *log.Logger
	ErrorLogger   *log.Logger
	initialized   bool
)

var logLevels = map[string]int{
	"debug": 1,
	"info":  2,
	"warn":  3,
	"error": 4,
}

var currentLevel int

// Init initializes the logger with configuration
func Init() {
	if initialized {
		return
	}

	// Create default loggers that will be reconfigured later
	InfoLogger = log.New(os.Stdout, "INFO: ", log.Ltime|log.Lshortfile)
	WarningLogger = log.New(os.Stdout, "WARN: ", log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(os.Stderr, "ERR: ", log.Ltime|log.Lshortfile)

	cfg := config.Get()
	level := strings.ToLower(cfg.Logging.Level)
	currentLevel = logLevels[level]
	if currentLevel == 0 {
		currentLevel = logLevels["info"]
	}

	initialized = true
}

func Info(v ...any) {
	if currentLevel <= logLevels["info"] {
		InfoLogger.Println(v...)
	}
}

func Warn(v ...any) {
	if currentLevel <= logLevels["warn"] {
		WarningLogger.Println(v...)
	}
}

func Err(v ...any) {
	if currentLevel <= logLevels["error"] {
		ErrorLogger.Println(v...)
	}
}
