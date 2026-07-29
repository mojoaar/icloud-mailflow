package db

import "database/sql"

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

func (r *LogRepo) Cleanup(keep int) error {
	_, err := r.DB.Exec(
		`DELETE FROM message_log WHERE id NOT IN (SELECT id FROM message_log ORDER BY id DESC LIMIT ?)`, keep,
	)
	return err
}
