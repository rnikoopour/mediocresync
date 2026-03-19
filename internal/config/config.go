package config

import (
	"os"
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
		dbPath = "./mediocresync.db"
	}

	return &Config{
		ListenAddr: listenAddr,
		DBPath:     dbPath,
		DevMode:    os.Getenv("DEV_MODE") == "true",
	}
}
