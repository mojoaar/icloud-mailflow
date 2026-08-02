package db

import "database/sql"

type SettingsRepo struct{ DB *sql.DB }

func NewSettingsRepo(d *sql.DB) *SettingsRepo {
	return &SettingsRepo{DB: d}
}

func (r *SettingsRepo) Get(key string) (string, error) {
	var v string
	err := r.DB.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func (r *SettingsRepo) Set(key, value string) error {
	_, err := r.DB.Exec(`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, key, value)
	return err
}
