package db

import (
	"database/sql"
	"fmt"
	"time"
)

const (
	metaAuthUsername     = "auth_username"
	metaAuthPasswordHash = "auth_password_hash"
)

type AuthRepository struct {
	db *sql.DB
}

func NewAuthRepository(db *sql.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

// GetCredentials returns the stored username and bcrypt password hash.
// configured is false when credentials have not yet been set up.
func (r *AuthRepository) GetCredentials() (username, passwordHash string, configured bool, err error) {
	var u, h string
	err = r.db.QueryRow(`SELECT value FROM _meta WHERE key = ?`, metaAuthUsername).Scan(&u)
	if err == sql.ErrNoRows {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("query auth username: %w", err)
	}
	err = r.db.QueryRow(`SELECT value FROM _meta WHERE key = ?`, metaAuthPasswordHash).Scan(&h)
	if err == sql.ErrNoRows {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("query auth password hash: %w", err)
	}
	return u, h, true, nil
}

// SetCredentials upserts the username and bcrypt password hash.
func (r *AuthRepository) SetCredentials(username, passwordHash string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, kv := range [][2]string{
		{metaAuthUsername, username},
		{metaAuthPasswordHash, passwordHash},
	} {
		_, err := tx.Exec(
			`INSERT INTO _meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
			kv[0], kv[1],
		)
		if err != nil {
			return fmt.Errorf("upsert %s: %w", kv[0], err)
		}
	}

	return tx.Commit()
}

// CreateSession inserts a new session token with last_used_at set to now.
func (r *AuthRepository) CreateSession(token string) error {
	_, err := r.db.Exec(
		`INSERT INTO sessions (token, last_used_at) VALUES (?, ?)`,
		token, formatTime(time.Now().UTC()),
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// LookupSession returns the last_used_at time for a session token.
// found is false when the token does not exist.
func (r *AuthRepository) LookupSession(token string) (lastUsedAt time.Time, found bool, err error) {
	var raw string
	err = r.db.QueryRow(`SELECT last_used_at FROM sessions WHERE token = ?`, token).Scan(&raw)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("lookup session: %w", err)
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("parse session time: %w", err)
	}
	return t, true, nil
}

// TouchSession updates last_used_at to now for the given token.
func (r *AuthRepository) TouchSession(token string) error {
	_, err := r.db.Exec(
		`UPDATE sessions SET last_used_at = ? WHERE token = ?`,
		formatTime(time.Now().UTC()), token,
	)
	if err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

// DeleteSession removes a single session token.
func (r *AuthRepository) DeleteSession(token string) error {
	_, err := r.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// SetSessionLastUsedAt overwrites the last_used_at timestamp for a session.
// Used in tests to simulate expired sessions.
func (r *AuthRepository) SetSessionLastUsedAt(token, lastUsedAt string) error {
	_, err := r.db.Exec(`UPDATE sessions SET last_used_at = ? WHERE token = ?`, lastUsedAt, token)
	if err != nil {
		return fmt.Errorf("set session last_used_at: %w", err)
	}
	return nil
}

// DeleteAllSessions removes all sessions, forcing re-login everywhere.
func (r *AuthRepository) DeleteAllSessions() error {
	_, err := r.db.Exec(`DELETE FROM sessions`)
	if err != nil {
		return fmt.Errorf("delete all sessions: %w", err)
	}
	return nil
}
