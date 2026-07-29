package db

import (
	"testing"
)

func TestSettingsSetAndGet(t *testing.T) {
	db := openTestDB(t)
	repo := NewSettingsRepo(db)

	if err := repo.Set("key1", "value1"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	v, err := repo.Get("key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v != "value1" {
		t.Errorf("Get = %q, want value1", v)
	}
}

func TestSettingsGetMissing(t *testing.T) {
	db := openTestDB(t)
	repo := NewSettingsRepo(db)

	v, err := repo.Get("missing")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v != "" {
		t.Errorf("Get missing key = %q, want empty", v)
	}
}

func TestSettingsGetAll(t *testing.T) {
	db := openTestDB(t)
	repo := NewSettingsRepo(db)

	repo.Set("a", "1")
	repo.Set("b", "2")

	all, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("GetAll len = %d, want 2", len(all))
	}
	if all["a"] != "1" {
		t.Errorf("all[a] = %q, want 1", all["a"])
	}
	if all["b"] != "2" {
		t.Errorf("all[b] = %q, want 2", all["b"])
	}
}

func TestSettingsOverwrite(t *testing.T) {
	db := openTestDB(t)
	repo := NewSettingsRepo(db)

	repo.Set("key", "old")
	repo.Set("key", "new")

	v, _ := repo.Get("key")
	if v != "new" {
		t.Errorf("Get = %q, want new", v)
	}
}
