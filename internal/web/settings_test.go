package web

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mojoaar/icloud-mailflow/internal/config"
	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func TestSettingsPage(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)
	foldersRepo := db.NewFoldersRepo(database)
	contactsRepo := db.NewContactsRepo(database)

	foldersRepo.Sync([]db.Folder{{Name: "INBOX", Path: "INBOX"}, {Name: "Archive", Path: "Archive"}})
	contactsRepo.Upsert("alice@example.com", "Alice")
	settingsRepo.Set("imap_email", "user@icloud.com")
	settingsRepo.Set("source_folder", "Processing")
	settingsRepo.Set("poll_interval", "300")
	settingsRepo.Set("poll_batch", "50")
	settingsRepo.Set("log_keep", "1000")
	settingsRepo.Set("timezone", "Europe/Copenhagen")

	cfg := config.Default()
	startTime = time.Now()

	h := settingsPage(settingsRepo, foldersRepo, cfg, nil, "0.4.2", contactsRepo)
	req := httptest.NewRequest("GET", "/settings", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"user@icloud.com", "INBOX", "Archive", "0.4.2"} {
		if !strings.Contains(body, want) {
			t.Errorf("body should contain %q", want)
		}
	}
}

func TestSettingsPageDefaults(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)
	foldersRepo := db.NewFoldersRepo(database)
	contactsRepo := db.NewContactsRepo(database)

	cfg := config.Default()
	startTime = time.Now()

	h := settingsPage(settingsRepo, foldersRepo, cfg, nil, "0.4.2", contactsRepo)
	req := httptest.NewRequest("GET", "/settings", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestDashboardHandler(t *testing.T) {
	database := openWebTestDB(t)
	rulesRepo := db.NewRulesRepo(database)
	foldersRepo := db.NewFoldersRepo(database)
	settingsRepo := db.NewSettingsRepo(database)
	contactsRepo := db.NewContactsRepo(database)

	rulesRepo.Create(&db.Rule{Name: "Rule 1", Priority: 0, Enabled: true})
	rulesRepo.Create(&db.Rule{Name: "Rule 2", Priority: 1, Enabled: false})
	foldersRepo.Sync([]db.Folder{{Name: "INBOX", Path: "INBOX"}})
	contactsRepo.Upsert("alice@example.com", "Alice")
	settingsRepo.Set("admin_password_hash", "hash")
	settingsRepo.Set("imap_email", "user@icloud.com")
	settingsRepo.Set("source_folder", "Processing")

	cfg := config.Default()

	h := dashboardHandler(&mockIMAPClient{}, nil, rulesRepo, foldersRepo, settingsRepo, contactsRepo, cfg)
	req := httptest.NewRequest("GET", "/dashboard", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Rule 1", "Rule 2"} {
		if !strings.Contains(body, want) {
			t.Errorf("body should contain %q", want)
		}
	}
}

func TestDashboardHandlerDisconnected(t *testing.T) {
	database := openWebTestDB(t)
	rulesRepo := db.NewRulesRepo(database)
	foldersRepo := db.NewFoldersRepo(database)
	settingsRepo := db.NewSettingsRepo(database)
	contactsRepo := db.NewContactsRepo(database)

	cfg := config.Default()

	h := dashboardHandler(nil, nil, rulesRepo, foldersRepo, settingsRepo, contactsRepo, cfg)
	req := httptest.NewRequest("GET", "/dashboard", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
