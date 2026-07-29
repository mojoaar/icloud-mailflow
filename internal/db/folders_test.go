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
