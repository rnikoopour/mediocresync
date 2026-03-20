package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	ListenAddr string
	DBPath     string
	DevMode    bool
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

	return &Config{
		ListenAddr: listenAddr,
		DBPath:     dbPath,
		DevMode:    os.Getenv("DEV_MODE") == "true",
	}
}
