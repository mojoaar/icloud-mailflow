package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	goimap "github.com/emersion/go-imap/v2"

	"github.com/mojoaar/icloud-mailflow/internal/contacts"
	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
)

func TestSeedContactsHandler(t *testing.T) {
	database := openWebTestDB(t)
	foldersRepo := db.NewFoldersRepo(database)
	contactsRepo := db.NewContactsRepo(database)
	foldersRepo.Sync([]db.Folder{{Name: "INBOX", Path: "INBOX"}})

	mock := &mockIMAPClient{
		searchUIDs: []goimap.UID{1},
		messages: map[uint32]*imap.Message{
			1: {From: []imap.Address{{Email: "seed@test.com"}}},
		},
	}

	collector := contacts.NewCollector(contactsRepo, mock)
	h := seedContactsHandler(collector, foldersRepo, contactsRepo)

	req := httptest.NewRequest("POST", "/contacts/seed", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	contacts, _ := contactsRepo.Search("seed@test.com")
	if len(contacts) != 1 {
		t.Fatalf("expected 1 contact seeded, got %d", len(contacts))
	}
}

func TestSeedContactsHandlerNilCollector(t *testing.T) {
	database := openWebTestDB(t)
	foldersRepo := db.NewFoldersRepo(database)
	contactsRepo := db.NewContactsRepo(database)

	h := seedContactsHandler(nil, foldersRepo, contactsRepo)

	req := httptest.NewRequest("POST", "/contacts/seed", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "IMAP not connected") {
		t.Fatalf("expected error message, got %q", body)
	}
}

func TestSeedContactsHandlerNoFolders(t *testing.T) {
	database := openWebTestDB(t)
	foldersRepo := db.NewFoldersRepo(database)
	contactsRepo := db.NewContactsRepo(database)

	mock := &mockIMAPClient{}
	collector := contacts.NewCollector(contactsRepo, mock)
	h := seedContactsHandler(collector, foldersRepo, contactsRepo)

	req := httptest.NewRequest("POST", "/contacts/seed", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "No folders synced") {
		t.Fatalf("expected 'No folders synced', got %q", body)
	}
}
