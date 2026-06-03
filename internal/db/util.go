package db

import (
	"database/sql"
	"time"
)

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func parseTimePtr(s *string) *time.Time {
	if s == nil {
		return nil
	}
	t, _ := time.Parse(time.RFC3339, *s)
	return &t
}

func parseNullTime(s sql.NullString) *time.Time {
	if !s.Valid {
		return nil
	}
	t, _ := time.Parse(time.RFC3339, s.String)
	return &t
}

