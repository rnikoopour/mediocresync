package config

import (
	"encoding/hex"
	"fmt"
	"os"
)

type Config struct {
	ListenAddr    string
	DBPath        string
	EncryptionKey []byte
	DevMode       bool
}

func Load() (*Config, error) {
	keyHex := os.Getenv("ENCRYPTION_KEY")
	if keyHex == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY environment variable is required")
	}

	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be a valid hex string: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must decode to exactly 32 bytes (got %d)", len(key))
	}

	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./mediocresync.db"
	}

	devMode := os.Getenv("DEV_MODE") == "true"

	return &Config{
		ListenAddr:    listenAddr,
		DBPath:        dbPath,
		EncryptionKey: key,
		DevMode:       devMode,
	}, nil
}
