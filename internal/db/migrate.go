package db

import "database/sql"

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
}

func Migrate(d *sql.DB) error {
	for _, m := range migrations {
		if _, err := d.Exec(m); err != nil {
			return err
		}
	}
	return nil
}
