package db

import (
	"database/sql"
	"log/slog"
)

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS folders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		path TEXT NOT NULL UNIQUE,
		flags TEXT NOT NULL DEFAULT '[]',
		synced_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS contacts (
		email TEXT PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		first_at TEXT NOT NULL DEFAULT (datetime('now')),
		last_at TEXT NOT NULL DEFAULT (datetime('now')),
		count INTEGER NOT NULL DEFAULT 1
	)`,
	`CREATE TABLE IF NOT EXISTS rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		priority INTEGER NOT NULL DEFAULT 0,
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS condition_groups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		rule_id INTEGER NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
		parent_id INTEGER REFERENCES condition_groups(id) ON DELETE CASCADE,
		logic_operator TEXT NOT NULL DEFAULT 'AND'
	)`,
	`CREATE TABLE IF NOT EXISTS conditions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		group_id INTEGER NOT NULL REFERENCES condition_groups(id) ON DELETE CASCADE,
		field TEXT NOT NULL,
		operator TEXT NOT NULL,
		value TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS actions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		rule_id INTEGER NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
		type TEXT NOT NULL,
		value TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		expires_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS message_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		uid INTEGER NOT NULL,
		subject TEXT NOT NULL DEFAULT '',
		from_addr TEXT NOT NULL DEFAULT '',
		rule_name TEXT NOT NULL DEFAULT '',
		action_type TEXT NOT NULL DEFAULT '',
		action_value TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'success'
	)`,
	`CREATE TABLE IF NOT EXISTS stats (
		category TEXT NOT NULL,
		key TEXT NOT NULL,
		value INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (category, key)
	)`,
	`CREATE TABLE IF NOT EXISTS auto_reply_log (
		recipient TEXT NOT NULL,
		reply_date TEXT NOT NULL,
		PRIMARY KEY (recipient, reply_date)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_condition_groups_rule_id ON condition_groups(rule_id)`,
	`CREATE INDEX IF NOT EXISTS idx_conditions_group_id ON conditions(group_id)`,
	`CREATE INDEX IF NOT EXISTS idx_actions_rule_id ON actions(rule_id)`,
}

func Migrate(d *sql.DB) error {
	for _, m := range migrations {
		if _, err := d.Exec(m); err != nil {
			return err
		}
	}
	slog.Debug("migrations complete")
	cols := []string{"schedule_days", "schedule_start", "schedule_end"}
	for _, col := range cols {
		var count int
		if err := d.QueryRow("SELECT COUNT(*) FROM pragma_table_info('rules') WHERE name=?", col).Scan(&count); err == nil && count == 0 {
			d.Exec("ALTER TABLE rules ADD COLUMN " + col + " TEXT NOT NULL DEFAULT ''")
		}
	}
	return backfillStats(d)
}

func backfillStats(d *sql.DB) error {
	var count int
	if err := d.QueryRow("SELECT COUNT(*) FROM stats").Scan(&count); err != nil {
		return nil
	}
	if count > 0 {
		return nil
	}

	var logCount int
	if err := d.QueryRow("SELECT COUNT(*) FROM message_log").Scan(&logCount); err != nil || logCount == 0 {
		return nil
	}

	queries := []string{
		`INSERT OR IGNORE INTO stats (category, key, value) SELECT 'total', 'processed', COUNT(*) FROM message_log`,
		`INSERT OR IGNORE INTO stats (category, key, value) SELECT 'rule_hit', rule_name, COUNT(*) FROM message_log WHERE rule_name != '' GROUP BY rule_name`,
		`INSERT OR IGNORE INTO stats (category, key, value) SELECT 'sender', from_addr, COUNT(*) FROM message_log WHERE from_addr != '' GROUP BY from_addr`,
		`INSERT OR IGNORE INTO stats (category, key, value) SELECT 'action', action_type, COUNT(*) FROM message_log WHERE action_type != '' GROUP BY action_type`,
		`INSERT OR IGNORE INTO stats (category, key, value) SELECT 'status', status, COUNT(*) FROM message_log GROUP BY status`,
		`INSERT OR IGNORE INTO stats (category, key, value) SELECT 'folder', action_value, COUNT(*) FROM message_log WHERE action_type='move_to_folder' AND action_value!='' GROUP BY action_value`,
		`INSERT OR IGNORE INTO stats (category, key, value) SELECT 'daily', date(created_at), COUNT(*) FROM message_log GROUP BY date(created_at)`,
		`INSERT OR IGNORE INTO stats (category, key, value) SELECT 'weekly', strftime('%G-W%V', created_at), COUNT(*) FROM message_log WHERE created_at != '' GROUP BY strftime('%G-W%V', created_at)`,
	}
	for _, q := range queries {
		if _, err := d.Exec(q); err != nil {
			return nil
		}
	}
	return nil
}
