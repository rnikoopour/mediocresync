package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type SourceRepository struct {
	db *sql.DB
}

func NewSourceRepository(db *sql.DB) *SourceRepository {
	return &SourceRepository{db: db}
}

func (r *SourceRepository) Create(s *Source) error {
	s.ID = uuid.New().String()
	now := time.Now().UTC()
	s.CreatedAt = now
	s.UpdatedAt = now

	var host, username sql.NullString
	var port sql.NullInt64
	var authType sql.NullString
	if s.Type == SourceTypeFTPES {
		host = sql.NullString{String: s.Host, Valid: s.Host != ""}
		port = sql.NullInt64{Int64: int64(s.Port), Valid: s.Port != 0}
		username = sql.NullString{String: s.Username, Valid: s.Username != ""}
	}
	if s.AuthType != "" {
		authType = sql.NullString{String: s.AuthType, Valid: true}
	}

	_, err := r.db.Exec(
		`INSERT INTO sources (id, name, type, host, port, username, password, skip_tls_verify, enable_epsv, auth_type, auth_credential, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Name, s.Type, host, port, username, s.Password,
		boolToInt(s.SkipTLSVerify), boolToInt(s.EnableEPSV),
		authType, s.AuthCredential,
		formatTime(s.CreatedAt), formatTime(s.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert source: %w", err)
	}
	return nil
}

func (r *SourceRepository) List() ([]*Source, error) {
	rows, err := r.db.Query(
		`SELECT id, name, type, host, port, username, password, skip_tls_verify, enable_epsv, auth_type, auth_credential, created_at, updated_at
		 FROM sources ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()

	var sources []*Source
	for rows.Next() {
		s, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, rows.Err()
}

func (r *SourceRepository) Get(id string) (*Source, error) {
	row := r.db.QueryRow(
		`SELECT id, name, type, host, port, username, password, skip_tls_verify, enable_epsv, auth_type, auth_credential, created_at, updated_at
		 FROM sources WHERE id = ?`, id,
	)
	s, err := scanSource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return s, err
}

func (r *SourceRepository) Update(s *Source) error {
	s.UpdatedAt = time.Now().UTC()

	var host, username sql.NullString
	var port sql.NullInt64
	var authType sql.NullString
	if s.Type == SourceTypeFTPES {
		host = sql.NullString{String: s.Host, Valid: s.Host != ""}
		port = sql.NullInt64{Int64: int64(s.Port), Valid: s.Port != 0}
		username = sql.NullString{String: s.Username, Valid: s.Username != ""}
	}
	if s.AuthType != "" {
		authType = sql.NullString{String: s.AuthType, Valid: true}
	}

	res, err := r.db.Exec(
		`UPDATE sources SET name=?, host=?, port=?, username=?, password=?, skip_tls_verify=?, enable_epsv=?, auth_type=?, auth_credential=?, updated_at=?
		 WHERE id=?`,
		s.Name, host, port, username, s.Password,
		boolToInt(s.SkipTLSVerify), boolToInt(s.EnableEPSV),
		authType, s.AuthCredential,
		formatTime(s.UpdatedAt), s.ID,
	)
	if err != nil {
		return fmt.Errorf("update source: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("source %s not found", s.ID)
	}
	return nil
}

func (r *SourceRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM sources WHERE id = ?`, id)
	return err
}

func scanSource(s scanner) (*Source, error) {
	var src Source
	var host, username, authType sql.NullString
	var port sql.NullInt64
	var skipTLS, enableEPSV int
	var createdAt, updatedAt string

	err := s.Scan(
		&src.ID, &src.Name, &src.Type,
		&host, &port, &username, &src.Password,
		&skipTLS, &enableEPSV,
		&authType, &src.AuthCredential,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan source: %w", err)
	}

	src.Host = host.String
	src.Port = int(port.Int64)
	src.Username = username.String
	src.SkipTLSVerify = skipTLS == 1
	src.EnableEPSV = enableEPSV == 1
	src.AuthType = authType.String
	src.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	src.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &src, nil
}
