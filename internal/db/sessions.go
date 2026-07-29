package db

import (
	"database/sql"
	"time"
)

type SessionsRepo struct{ DB *sql.DB }

func NewSessionsRepo(d *sql.DB) *SessionsRepo { return &SessionsRepo{DB: d} }

func (r *SessionsRepo) Create(token string, ttl time.Duration) error {
	expires := time.Now().Add(ttl).UTC().Format(time.RFC3339)
	_, err := r.DB.Exec(`INSERT INTO sessions (token, expires_at) VALUES (?, ?)`, token, expires)
	return err
}

func (r *SessionsRepo) Validate(token string) (bool, error) {
	var expires string
	err := r.DB.QueryRow(`SELECT expires_at FROM sessions WHERE token = ?`, token).Scan(&expires)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	t, err := time.Parse(time.RFC3339, expires)
	if err != nil {
		return false, err
	}
	return time.Now().Before(t), nil
}

func (r *SessionsRepo) Delete(token string) error {
	_, err := r.DB.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func (r *SessionsRepo) Cleanup() error {
	_, err := r.DB.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now().UTC().Format(time.RFC3339))
	return err
}
