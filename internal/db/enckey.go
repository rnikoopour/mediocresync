package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
)

const encKeyMeta = "encryption_key"

// GetOrCreateEncryptionKey returns the AES-256 encryption key stored in the
// _meta table, generating and persisting a new random key on first startup.
func GetOrCreateEncryptionKey(db *sql.DB) ([]byte, error) {
	var hexKey string
	err := db.QueryRow(`SELECT value FROM _meta WHERE key = ?`, encKeyMeta).Scan(&hexKey)
	if err == nil {
		key, err := hex.DecodeString(hexKey)
		if err != nil {
			return nil, fmt.Errorf("decode stored encryption key: %w", err)
		}
		return key, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("query encryption key: %w", err)
	}

	// First run — generate a new 32-byte key.
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate encryption key: %w", err)
	}

	_, err = db.Exec(`INSERT INTO _meta (key, value) VALUES (?, ?)`, encKeyMeta, hex.EncodeToString(key))
	if err != nil {
		return nil, fmt.Errorf("store encryption key: %w", err)
	}

	return key, nil
}
