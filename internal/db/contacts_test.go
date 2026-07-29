package db

import (
	"testing"
)

func TestContactsUpsert(t *testing.T) {
	db := openTestDB(t)
	repo := NewContactsRepo(db)

	if err := repo.Upsert("alice@example.com", "Alice"); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	contacts, err := repo.Search("alice")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(contacts) != 1 {
		t.Fatalf("len = %d, want 1", len(contacts))
	}
	if contacts[0].Email != "alice@example.com" {
		t.Errorf("Email = %q", contacts[0].Email)
	}
	if contacts[0].Name != "Alice" {
		t.Errorf("Name = %q", contacts[0].Name)
	}
	if contacts[0].Count != 1 {
		t.Errorf("Count = %d, want 1", contacts[0].Count)
	}
}

func TestContactsUpsertIncrementsCount(t *testing.T) {
	db := openTestDB(t)
	repo := NewContactsRepo(db)

	repo.Upsert("bob@example.com", "Bob")
	repo.Upsert("bob@example.com", "Bob")

	contacts, _ := repo.Search("bob")
	if contacts[0].Count != 2 {
		t.Errorf("Count = %d, want 2", contacts[0].Count)
	}
}

func TestContactsSearchByName(t *testing.T) {
	db := openTestDB(t)
	repo := NewContactsRepo(db)

	repo.Upsert("alice@example.com", "Alice Smith")
	repo.Upsert("bob@example.com", "Bob Jones")

	contacts, err := repo.Search("smith")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(contacts) != 1 {
		t.Errorf("len = %d, want 1", len(contacts))
	}
}

func TestContactsSearchNoMatch(t *testing.T) {
	db := openTestDB(t)
	repo := NewContactsRepo(db)

	repo.Upsert("alice@example.com", "Alice")

	contacts, err := repo.Search("xyzzy")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(contacts) != 0 {
		t.Errorf("len = %d, want 0", len(contacts))
	}
}

func TestContactsUpsertBatch(t *testing.T) {
	db := openTestDB(t)
	repo := NewContactsRepo(db)

	entries := []Contact{
		{Email: "a@b.com", Name: "A"},
		{Email: "c@d.com", Name: "C"},
		{Email: "e@f.com", Name: "E"},
	}
	if err := repo.UpsertBatch(entries); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	contacts, err := repo.Search("b.com")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(contacts) < 1 {
		t.Error("should find at least one contact")
	}
}

func TestContactsUpsertBatchUpdatesExisting(t *testing.T) {
	db := openTestDB(t)
	repo := NewContactsRepo(db)

	repo.Upsert("x@y.com", "Old")
	repo.UpsertBatch([]Contact{{Email: "x@y.com", Name: "New"}})

	contacts, _ := repo.Search("x@y.com")
	if contacts[0].Name != "New" {
		t.Errorf("Name = %q, want New", contacts[0].Name)
	}
	if contacts[0].Count < 2 {
		t.Errorf("Count = %d, want >= 2", contacts[0].Count)
	}
}

func TestCount(t *testing.T) {
	db := openTestDB(t)
	repo := NewContactsRepo(db)

	count, err := repo.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Errorf("initial count = %d, want 0", count)
	}

	repo.Upsert("a@x.com", "")
	repo.Upsert("b@x.com", "")

	count, _ = repo.Count()
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}
