package db

import "database/sql"

type Contact struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	FirstAt string `json:"first_at"`
	LastAt  string `json:"last_at"`
	Count   int64  `json:"count"`
}

type ContactsRepo struct{ DB *sql.DB }

func NewContactsRepo(d *sql.DB) *ContactsRepo {
	return &ContactsRepo{DB: d}
}

func (r *ContactsRepo) Upsert(email, name string) error {
	_, err := r.DB.Exec(
		`INSERT INTO contacts (email, name, first_at, last_at, count) VALUES (?, ?, datetime('now'), datetime('now'), 1)
		ON CONFLICT(email) DO UPDATE SET name = excluded.name, last_at = excluded.last_at, count = contacts.count + 1`,
		email, name,
	)
	return err
}

func (r *ContactsRepo) Search(q string) ([]Contact, error) {
	rows, err := r.DB.Query(
		`SELECT email, name, first_at, last_at, count FROM contacts
		WHERE email LIKE ? OR name LIKE ? ORDER BY count DESC LIMIT 20`,
		"%"+q+"%", "%"+q+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Contact
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.Email, &c.Name, &c.FirstAt, &c.LastAt, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *ContactsRepo) Count() (int, error) {
	var count int
	err := r.DB.QueryRow(`SELECT COUNT(*) FROM contacts`).Scan(&count)
	return count, err
}

func (r *ContactsRepo) UpsertBatch(entries []Contact) error {
	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range entries {
		_, err := tx.Exec(
			`INSERT INTO contacts (email, name, first_at, last_at, count) VALUES (?, ?, datetime('now'), datetime('now'), 1)
			ON CONFLICT(email) DO UPDATE SET name = excluded.name, last_at = excluded.last_at, count = contacts.count + 1`,
			c.Email, c.Name,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *ContactsRepo) ListAll() ([]Contact, error) {
	rows, err := r.DB.Query(`SELECT email, name, first_at, last_at, count FROM contacts ORDER BY count DESC, email ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Contact
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.Email, &c.Name, &c.FirstAt, &c.LastAt, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
