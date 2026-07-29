package db

import (
	"database/sql"
	"testing"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigrateCreatesAllTables(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	defer rows.Close()

	tables := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		tables[name] = true
	}

	expected := []string{"actions", "condition_groups", "conditions", "contacts", "folders", "message_log", "rules", "sessions", "settings"}
	for _, table := range expected {
		if !tables[table] {
			t.Errorf("table %q not found", table)
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}
