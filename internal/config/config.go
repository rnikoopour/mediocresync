package config

import (
	"os"
	"path/filepath"
)

type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

type Config struct {
	ListenAddr string
	DBPath     string
	LogFile    string
	DevMode    bool
	LogLevel   LogLevel
}

func Load() *Config {
	listenAddr := os.Getenv("MEDIOCRESYNC_LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
	}

	dbPath := os.Getenv("MEDIOCRESYNC_DB_PATH")
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		dbPath = filepath.Join(home, ".mediocresync", "mediocresync.db")
	}

	logLevel := LogLevel(os.Getenv("MEDIOCRESYNC_LOG_LEVEL"))
	switch logLevel {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
	default:
		logLevel = LogLevelInfo
	}

	logFile := os.Getenv("MEDIOCRESYNC_LOG_FILE")
	if logFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		logFile = filepath.Join(home, ".mediocresync", "mediocresync.log")
	}

	return &Config{
		ListenAddr: listenAddr,
		DBPath:     dbPath,
		LogFile:    logFile,
		DevMode:    os.Getenv("MEDIOCRESYNC_DEV_MODE") == "true",
		LogLevel:   logLevel,
	}
}
