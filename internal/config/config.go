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
	DevMode    bool
	LogLevel   LogLevel
}

func Load() *Config {
	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		dbPath = filepath.Join(home, ".mediocresync", "mediocresync.db")
	}

	logLevel := LogLevel(os.Getenv("LOG_LEVEL"))
	switch logLevel {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
	default:
		logLevel = LogLevelInfo
	}

	return &Config{
		ListenAddr: listenAddr,
		DBPath:     dbPath,
		DevMode:    os.Getenv("DEV_MODE") == "true",
		LogLevel:   logLevel,
	}
}
