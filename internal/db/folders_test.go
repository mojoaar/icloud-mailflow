package db

import (
	"testing"
)

func TestFoldersListEmpty(t *testing.T) {
	db := openTestDB(t)
	repo := NewFoldersRepo(db)

	folders, err := repo.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(folders) != 0 {
		t.Errorf("len = %d, want 0", len(folders))
	}
}

func TestFoldersSyncAndList(t *testing.T) {
	db := openTestDB(t)
	repo := NewFoldersRepo(db)

	input := []Folder{
		{Name: "INBOX", Path: "INBOX", Flags: `\HasNoChildren`},
		{Name: "Sent", Path: "Sent", Flags: `\Sent`},
		{Name: "Trash", Path: "Trash", Flags: `\Trash`},
	}
	if err := repo.Sync(input); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	folders, err := repo.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(folders) != 3 {
		t.Fatalf("len = %d, want 3", len(folders))
	}

	names := map[string]bool{}
	for _, f := range folders {
		names[f.Name] = true
	}
	for _, name := range []string{"INBOX", "Sent", "Trash"} {
		if !names[name] {
			t.Errorf("folder %q not found", name)
		}
	}
}

func TestFoldersSyncReplacesAll(t *testing.T) {
	db := openTestDB(t)
	repo := NewFoldersRepo(db)

	repo.Sync([]Folder{{Name: "Old", Path: "Old", Flags: ""}})
	repo.Sync([]Folder{{Name: "New", Path: "New", Flags: ""}})

	folders, _ := repo.List()
	if len(folders) != 1 {
		t.Errorf("len = %d, want 1", len(folders))
	}
	if folders[0].Name != "New" {
		t.Errorf("Name = %q, want New", folders[0].Name)
	}
}

func TestFoldersSyncDedupes(t *testing.T) {
	d := NewTestDB(t)
	repo := NewFoldersRepo(d)
	if err := repo.Sync([]Folder{{Name: "A", Path: "A"}, {Name: "A", Path: "A"}}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	list, err := repo.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("len = %d, want 1", len(list))
	}
}

func TestFoldersSyncPreservesIDsAndRemovesMissing(t *testing.T) {
	d := NewTestDB(t)
	repo := NewFoldersRepo(d)
	if err := repo.Sync([]Folder{{Name: "A", Path: "A"}, {Name: "B", Path: "B"}}); err != nil {
		t.Fatalf("Sync 1: %v", err)
	}
	before, _ := repo.List()
	idOf := func(list []Folder, path string) int64 {
		for _, f := range list {
			if f.Path == path {
				return f.ID
			}
		}
		return 0
	}
	idA := idOf(before, "A")
	if idA == 0 {
		t.Fatal("folder A not found")
	}

	if err := repo.Sync([]Folder{{Name: "A", Path: "A", Flags: "\\Flagged"}, {Name: "C", Path: "C"}}); err != nil {
		t.Fatalf("Sync 2: %v", err)
	}
	after, _ := repo.List()
	if idOf(after, "A") != idA {
		t.Errorf("folder A id changed: %d -> %d", idA, idOf(after, "A"))
	}
	if idOf(after, "B") != 0 {
		t.Error("folder B should have been removed")
	}
	if idOf(after, "C") == 0 {
		t.Error("folder C should have been added")
	}
	for _, f := range after {
		if f.Path == "A" && f.Flags != "\\Flagged" {
			t.Errorf("folder A flags = %q, want \\Flagged", f.Flags)
		}
	}
}
