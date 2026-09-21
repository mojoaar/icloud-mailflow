package db

import "testing"

func TestMigrateBackfillsStats(t *testing.T) {
	database := NewTestDB(t)
	if _, err := database.Exec(`INSERT INTO message_log (uid, subject, from_addr, rule_name, action_type, status)
		VALUES (1, 's', 'a@b.com', 'r1', 'move_to_folder', 'success')`); err != nil {
		t.Fatalf("insert log: %v", err)
	}
	if _, err := database.Exec(`DELETE FROM stats`); err != nil {
		t.Fatalf("clear stats: %v", err)
	}
	if err := Migrate(database); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	var n int
	if err := database.QueryRow(`SELECT COALESCE((SELECT value FROM stats WHERE category='total' AND key='processed'),0)`).Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 1 {
		t.Errorf("total/processed = %d, want 1 (backfill did not run)", n)
	}
}
