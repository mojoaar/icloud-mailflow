package db

import (
	"testing"
	"time"
)

func TestSessionsCreateAndValidate(t *testing.T) {
	db := openTestDB(t)
	repo := NewSessionsRepo(db)

	token := "test-token-123"
	if err := repo.Create(token, time.Hour); err != nil {
		t.Fatalf("Create: %v", err)
	}

	valid, err := repo.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !valid {
		t.Error("token should be valid")
	}
}

func TestSessionsValidateMissing(t *testing.T) {
	db := openTestDB(t)
	repo := NewSessionsRepo(db)

	valid, err := repo.Validate("nonexistent")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if valid {
		t.Error("missing token should not be valid")
	}
}

func TestSessionsValidateExpired(t *testing.T) {
	db := openTestDB(t)
	repo := NewSessionsRepo(db)

	token := "expired-token"
	if err := repo.Create(token, -time.Hour); err != nil {
		t.Fatalf("Create: %v", err)
	}

	valid, err := repo.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if valid {
		t.Error("expired token should not be valid")
	}
}

func TestSessionsDelete(t *testing.T) {
	db := openTestDB(t)
	repo := NewSessionsRepo(db)

	token := "delete-me"
	if err := repo.Create(token, time.Hour); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Delete(token); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	valid, err := repo.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if valid {
		t.Error("deleted token should not be valid")
	}
}

func TestSessionsCleanup(t *testing.T) {
	db := openTestDB(t)
	repo := NewSessionsRepo(db)

	repo.Create("valid-token", time.Hour)
	repo.Create("expired-token", -time.Hour)

	if err := repo.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	valid, _ := repo.Validate("valid-token")
	if !valid {
		t.Error("valid token should survive cleanup")
	}

	valid, _ = repo.Validate("expired-token")
	if valid {
		t.Error("expired token should be cleaned up")
	}
}

func TestSessionsDeleteAllExcept(t *testing.T) {
	d := NewTestDB(t)
	repo := NewSessionsRepo(d)
	if err := repo.Create("keep", time.Hour); err != nil {
		t.Fatalf("Create keep: %v", err)
	}
	if err := repo.Create("drop", time.Hour); err != nil {
		t.Fatalf("Create drop: %v", err)
	}
	if err := repo.DeleteAllExcept("keep"); err != nil {
		t.Fatalf("DeleteAllExcept: %v", err)
	}
	if ok, _ := repo.Validate("keep"); !ok {
		t.Error("keep session should remain valid")
	}
	if ok, _ := repo.Validate("drop"); ok {
		t.Error("drop session should be deleted")
	}
}
