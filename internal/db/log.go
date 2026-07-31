package db

import (
	"database/sql"
	"fmt"
	"strings"
)

type LogEntry struct {
	ID          int64  `json:"id"`
	CreatedAt   string `json:"created_at"`
	UID         int64  `json:"uid"`
	Subject     string `json:"subject"`
	FromAddr    string `json:"from_addr"`
	RuleName    string `json:"rule_name"`
	ActionType  string `json:"action_type"`
	ActionValue string `json:"action_value"`
	Status      string `json:"status"`
}

type LogRepo struct{ DB *sql.DB }

func NewLogRepo(d *sql.DB) *LogRepo {
	return &LogRepo{DB: d}
}

func (r *LogRepo) Insert(entry *LogEntry) error {
	_, err := r.DB.Exec(
		`INSERT INTO message_log (uid, subject, from_addr, rule_name, action_type, action_value, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.UID, entry.Subject, entry.FromAddr, entry.RuleName,
		entry.ActionType, entry.ActionValue, entry.Status,
	)
	return err
}

func (r *LogRepo) ListRecent(limit int) ([]LogEntry, error) {
	rows, err := r.DB.Query(
		`SELECT id, created_at, uid, subject, from_addr, rule_name, action_type, action_value, status
		FROM message_log ORDER BY id DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LogEntry
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.UID, &e.Subject, &e.FromAddr,
			&e.RuleName, &e.ActionType, &e.ActionValue, &e.Status); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *LogRepo) DeleteAll() error {
	_, err := r.DB.Exec(`DELETE FROM message_log`)
	return err
}

func (r *LogRepo) ListFiltered(limit, offset int, search, rule, status string) ([]LogEntry, int, error) {
	var wheres []string
	var args []any

	if search != "" {
		s := "%" + search + "%"
		wheres = append(wheres, "(subject LIKE ? OR from_addr LIKE ? OR rule_name LIKE ?)")
		args = append(args, s, s, s)
	}
	if rule != "" {
		wheres = append(wheres, "rule_name = ?")
		args = append(args, rule)
	}
	if status != "" {
		wheres = append(wheres, "status = ?")
		args = append(args, status)
	}

	where := ""
	if len(wheres) > 0 {
		where = "WHERE " + strings.Join(wheres, " AND ")
	}

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	row := r.DB.QueryRow("SELECT COUNT(*) FROM message_log "+where, countArgs...)
	if err := row.Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf("SELECT id, created_at, uid, subject, from_addr, rule_name, action_type, action_value, status FROM message_log %s ORDER BY id DESC LIMIT ? OFFSET ?", where)
	args = append(args, limit, offset)
	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []LogEntry
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.UID, &e.Subject, &e.FromAddr,
			&e.RuleName, &e.ActionType, &e.ActionValue, &e.Status); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func (r *LogRepo) Cleanup(keep int) error {
	_, err := r.DB.Exec(
		`DELETE FROM message_log WHERE id NOT IN (SELECT id FROM message_log ORDER BY id DESC LIMIT ?)`, keep,
	)
	return err
}
