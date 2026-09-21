package db

import (
	"testing"
	"time"
)

func TestAutoReplyPrune(t *testing.T) {
	d := NewTestDB(t)
	repo := NewAutoReplyRepo(d)

	old := time.Now().AddDate(0, 0, -10).Format("2006-01-02")
	today := time.Now().Format("2006-01-02")
	if _, err := d.Exec(`INSERT INTO auto_reply_log (recipient, reply_date) VALUES ('old@x.com', ?), ('new@x.com', ?)`, old, today); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := repo.Prune(time.Now().AddDate(0, 0, -7)); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM auto_reply_log`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("rows = %d, want 1 (old row pruned)", n)
	}
}
