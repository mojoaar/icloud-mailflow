package db

import "testing"

func TestAuditRepoAddList(t *testing.T) {
	repo := NewAuditRepo(NewTestDB(t))

	if err := repo.Add("login", "admin", "", "1.2.3.4"); err != nil {
		t.Fatalf("Add login: %v", err)
	}
	if err := repo.Add("rule_created", "admin", "Shopping", "1.2.3.4"); err != nil {
		t.Fatalf("Add rule_created: %v", err)
	}

	entries, total, err := repo.ListFiltered(100, 0, "")
	if err != nil {
		t.Fatalf("ListFiltered: %v", err)
	}
	if total != 2 || len(entries) != 2 {
		t.Fatalf("total=%d len=%d, want 2/2", total, len(entries))
	}
	if entries[0].Action != "rule_created" || entries[1].Action != "login" {
		t.Errorf("unexpected order: %v", entries)
	}

	entries, total, err = repo.ListFiltered(100, 0, "login")
	if err != nil || total != 1 || len(entries) != 1 || entries[0].Action != "login" {
		t.Errorf("filter: total=%d len=%d err=%v", total, len(entries), err)
	}
}

func TestAuditRepoCleanup(t *testing.T) {
	repo := NewAuditRepo(NewTestDB(t))
	for i := 0; i < 5; i++ {
		if err := repo.Add("login", "", "", ""); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	if err := repo.Cleanup(2); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	_, total, _ := repo.ListFiltered(100, 0, "")
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
}
