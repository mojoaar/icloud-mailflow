package db

import (
	"database/sql"
	"testing"
)

func TestLogRepo_ListFiltered(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.Exec(`CREATE TABLE message_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at TEXT DEFAULT (datetime('now')),
		uid INTEGER, subject TEXT, from_addr TEXT,
		rule_name TEXT, action_type TEXT, action_value TEXT, status TEXT,
		folder TEXT NOT NULL DEFAULT ''
	)`)
	repo := &LogRepo{DB: db}

	entries := []LogEntry{
		{UID: 1, Subject: "Hello", FromAddr: "a@b.com", RuleName: "Test", ActionType: "move_to_folder", Status: "success"},
		{UID: 2, Subject: "World", FromAddr: "c@d.com", RuleName: "Other", ActionType: "mark_as_read", Status: "error"},
		{UID: 3, Subject: "Foo", FromAddr: "a@b.com", RuleName: "Test", ActionType: "set_flag", ActionValue: "\\Seen", Status: "success"},
	}
	for _, e := range entries {
		repo.Insert(&e)
	}

	t.Run("no filters", func(t *testing.T) {
		results, total, err := repo.ListFiltered(10, 0, "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
		if len(results) != 3 {
			t.Errorf("len = %d, want 3", len(results))
		}
	})

	t.Run("search", func(t *testing.T) {
		_, total, err := repo.ListFiltered(10, 0, "Hello", "", "")
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 {
			t.Errorf("total = %d, want 1", total)
		}
	})

	t.Run("rule filter", func(t *testing.T) {
		_, total, err := repo.ListFiltered(10, 0, "", "Test", "")
		if err != nil {
			t.Fatal(err)
		}
		if total != 2 {
			t.Errorf("total = %d, want 2", total)
		}
	})

	t.Run("status filter", func(t *testing.T) {
		_, total, err := repo.ListFiltered(10, 0, "", "", "error")
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 {
			t.Errorf("total = %d, want 1", total)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		results, total, err := repo.ListFiltered(1, 0, "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 {
			t.Errorf("len = %d, want 1", len(results))
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
	})
}

func TestLogRepo_DeleteByIDs(t *testing.T) {
	d := NewTestDB(t)
	repo := NewLogRepo(d)
	for i := 0; i < 3; i++ {
		if err := repo.Insert(&LogEntry{UID: int64(i + 1), Subject: "s"}); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}
	entries, err := repo.ListRecent(10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("setup: got %d entries, want 3", len(entries))
	}

	n, err := repo.DeleteByIDs([]int64{entries[0].ID, entries[2].ID})
	if err != nil {
		t.Fatalf("DeleteByIDs: %v", err)
	}
	if n != 2 {
		t.Errorf("deleted = %d, want 2", n)
	}
	left, err := repo.ListRecent(10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(left) != 1 || left[0].ID != entries[1].ID {
		t.Errorf("remaining = %+v, want only id %d", left, entries[1].ID)
	}

	if n, err := repo.DeleteByIDs(nil); err != nil || n != 0 {
		t.Errorf("DeleteByIDs(nil) = (%d, %v), want (0, nil)", n, err)
	}
}

func TestLogEntryFolderRoundTrip(t *testing.T) {
	d := NewTestDB(t)
	repo := NewLogRepo(d)
	if err := repo.Insert(&LogEntry{UID: 1, Subject: "s", Folder: "Archive", Status: "success"}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	entries, err := repo.ListRecent(1)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(entries) != 1 || entries[0].Folder != "Archive" {
		t.Errorf("folder = %+v, want Archive", entries)
	}
}
