package db

import "testing"

func TestAutoReplyRepoShouldReply(t *testing.T) {
	database := NewTestDB(t)
	repo := NewAutoReplyRepo(database)

	ok, err := repo.ShouldReply("a@test.com")
	if err != nil {
		t.Fatalf("ShouldReply: %v", err)
	}
	if !ok {
		t.Fatal("first call should return true")
	}

	ok, err = repo.ShouldReply("a@test.com")
	if err != nil {
		t.Fatalf("ShouldReply: %v", err)
	}
	if ok {
		t.Fatal("second same-day call should return false")
	}

	ok, err = repo.ShouldReply("b@test.com")
	if err != nil {
		t.Fatalf("ShouldReply: %v", err)
	}
	if !ok {
		t.Fatal("different recipient should return true")
	}
}
