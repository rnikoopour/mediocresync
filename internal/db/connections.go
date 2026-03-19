package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ConnectionRepository struct {
	db *sql.DB
}

func NewConnectionRepository(db *sql.DB) *ConnectionRepository {
	return &ConnectionRepository{db: db}
}

func (r *ConnectionRepository) Create(c *Connection) error {
	c.ID = uuid.New().String()
	now := time.Now().UTC()
	c.CreatedAt = now
	c.UpdatedAt = now

	_, err := r.db.Exec(
		`INSERT INTO connections (id, name, host, port, username, password, skip_tls_verify, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Host, c.Port, c.Username, c.Password,
		boolToInt(c.SkipTLSVerify), formatTime(c.CreatedAt), formatTime(c.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert connection: %w", err)
	}
	return nil
}

func (r *ConnectionRepository) List() ([]*Connection, error) {
	rows, err := r.db.Query(
		`SELECT id, name, host, port, username, password, skip_tls_verify, created_at, updated_at
		 FROM connections ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	defer rows.Close()

	var conns []*Connection
	for rows.Next() {
		c, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		conns = append(conns, c)
	}
	return conns, rows.Err()
}

func (r *ConnectionRepository) Get(id string) (*Connection, error) {
	row := r.db.QueryRow(
		`SELECT id, name, host, port, username, password, skip_tls_verify, created_at, updated_at
		 FROM connections WHERE id = ?`, id,
	)
	c, err := scanConnection(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

func (r *ConnectionRepository) Update(c *Connection) error {
	c.UpdatedAt = time.Now().UTC()
	res, err := r.db.Exec(
		`UPDATE connections SET name=?, host=?, port=?, username=?, password=?, skip_tls_verify=?, updated_at=?
		 WHERE id=?`,
		c.Name, c.Host, c.Port, c.Username, c.Password,
		boolToInt(c.SkipTLSVerify), formatTime(c.UpdatedAt), c.ID,
	)
	if err != nil {
		return fmt.Errorf("update connection: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("connection %s not found", c.ID)
	}
	return nil
}

func (r *ConnectionRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM connections WHERE id = ?`, id)
	return err
}

// scanner is satisfied by both *sql.Row and *sql.Rows
type scanner interface {
	Scan(dest ...any) error
}

func scanConnection(s scanner) (*Connection, error) {
	var c Connection
	var skipTLS int
	var createdAt, updatedAt string

	err := s.Scan(
		&c.ID, &c.Name, &c.Host, &c.Port, &c.Username, &c.Password,
		&skipTLS, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan connection: %w", err)
	}

	c.SkipTLSVerify = skipTLS == 1
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &c, nil
}
