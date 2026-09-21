package db

import (
	"database/sql"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	dsn := path + "?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=1"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if isMemoryDSN(path) {
		// An in-memory database is private per connection, so keep a single one.
		db.SetMaxOpenConns(1)
		return db, nil
	}
	// WAL allows concurrent readers alongside a single writer; busy_timeout
	// handles the brief write contention. Reader/writer mixing is covered by
	// TestConcurrentReadWrite.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	return db, nil
}

func isMemoryDSN(path string) bool {
	return path == ":memory:" || strings.Contains(path, "mode=memory") || strings.Contains(path, "cache=shared")
}
