package db

import (
	"database/sql"
	"fmt"
)

type AuditEntry struct {
	ID        int64  `json:"id"`
	CreatedAt string `json:"created_at"`
	Action    string `json:"action"`
	Actor     string `json:"actor"`
	Detail    string `json:"detail"`
	IP        string `json:"ip"`
}

type AuditRepo struct{ DB *sql.DB }

func NewAuditRepo(d *sql.DB) *AuditRepo {
	return &AuditRepo{DB: d}
}

func (r *AuditRepo) Add(action, actor, detail, ip string) error {
	_, err := r.DB.Exec(
		`INSERT INTO audit_log (action, actor, detail, ip) VALUES (?, ?, ?, ?)`,
		action, actor, detail, ip,
	)
	return err
}

func (r *AuditRepo) ListFiltered(limit, offset int, action string) ([]AuditEntry, int, error) {
	where := ""
	args := []any{}
	if action != "" {
		where = "WHERE action = ?"
		args = append(args, action)
	}

	var total int
	if err := r.DB.QueryRow("SELECT COUNT(*) FROM audit_log "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf("SELECT id, created_at, action, actor, detail, ip FROM audit_log %s ORDER BY id DESC LIMIT ? OFFSET ?", where)
	args = append(args, limit, offset)
	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.Action, &e.Actor, &e.Detail, &e.IP); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func (r *AuditRepo) DeleteAll() error {
	_, err := r.DB.Exec(`DELETE FROM audit_log`)
	return err
}

func (r *AuditRepo) Cleanup(keep int) error {
	_, err := r.DB.Exec(
		`DELETE FROM audit_log WHERE id NOT IN (SELECT id FROM audit_log ORDER BY id DESC LIMIT ?)`, keep,
	)
	return err
}
