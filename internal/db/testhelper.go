package db

import (
	"database/sql"
	"testing"
)

func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := Migrate(database); err != nil {
		database.Close()
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}
