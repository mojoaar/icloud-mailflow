package db

import "testing"

func TestLogInsertAndList(t *testing.T) {
	d := NewTestDB(t)
	repo := NewLogRepo(d)

	e1 := &LogEntry{UID: 1, Subject: "Hello", FromAddr: "a@b.com", RuleName: "test", ActionType: "move", ActionValue: "Trash", Status: "success"}
	e2 := &LogEntry{UID: 2, Subject: "World", FromAddr: "c@d.com", RuleName: "spam", ActionType: "mark_as_read", ActionValue: "", Status: "error"}

	if err := repo.Insert(e1); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := repo.Insert(e2); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	entries, err := repo.ListRecent(10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len = %d, want 2", len(entries))
	}
	if entries[0].Subject != "World" {
		t.Errorf("most recent subject = %q, want World", entries[0].Subject)
	}
	if entries[1].Subject != "Hello" {
		t.Errorf("older subject = %q, want Hello", entries[1].Subject)
	}
}

func TestLogListRecentLimit(t *testing.T) {
	d := NewTestDB(t)
	repo := NewLogRepo(d)

	for i := 0; i < 5; i++ {
		repo.Insert(&LogEntry{UID: int64(i), Subject: "test"})
	}
	entries, err := repo.ListRecent(3)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3", len(entries))
	}
}

func TestLogDeleteAll(t *testing.T) {
	d := NewTestDB(t)
	repo := NewLogRepo(d)

	repo.Insert(&LogEntry{UID: 1, Subject: "test"})
	repo.Insert(&LogEntry{UID: 2, Subject: "test"})

	if err := repo.DeleteAll(); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	entries, _ := repo.ListRecent(10)
	if len(entries) != 0 {
		t.Errorf("expected empty after DeleteAll, got %d", len(entries))
	}
}

func TestLogCleanup(t *testing.T) {
	d := NewTestDB(t)
	repo := NewLogRepo(d)

	for i := 0; i < 5; i++ {
		repo.Insert(&LogEntry{UID: int64(i), Subject: "test"})
	}
	if err := repo.Cleanup(2); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	entries, _ := repo.ListRecent(10)
	if len(entries) != 2 {
		t.Errorf("len = %d, want 2 after cleanup(2)", len(entries))
	}
}
