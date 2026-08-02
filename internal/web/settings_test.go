package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mojoaar/icloud-mailflow/internal/config"
	"github.com/mojoaar/icloud-mailflow/internal/db"
)

const testHexKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestSetupLockedWhenPasswordSetEmailEmpty(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)
	settingsRepo.Set("admin_password_hash", "existinghash")

	h := setupPage(settingsRepo, database, &config.Config{})

	req := httptest.NewRequest("GET", "/setup", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("GET status = %d, want 303", rec.Code)
	}

	form := url.Values{"password": {"attacker"}}
	preq := httptest.NewRequest("POST", "/setup", strings.NewReader(form.Encode()))
	preq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	prec := httptest.NewRecorder()
	h.ServeHTTP(prec, preq)

	if got, _ := settingsRepo.Get("admin_password_hash"); got != "existinghash" {
		t.Errorf("admin_password_hash overwritten: got %q", got)
	}
}

func TestSetupOpenWhenNoPassword(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)

	h := setupPage(settingsRepo, database, &config.Config{})
	req := httptest.NewRequest("GET", "/setup", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestEncryptPasswordEmptyKeyErrors(t *testing.T) {
	out, err := encryptPassword("secret", "")
	if err == nil {
		t.Fatal("expected error with empty key")
	}
	if out == "secret" {
		t.Error("must never return plaintext on error")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	enc, err := encryptPassword("hunter2", testHexKey)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if enc == "hunter2" {
		t.Fatal("ciphertext equals plaintext")
	}
	dec, err := decryptPassword(enc, testHexKey)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if dec != "hunter2" {
		t.Errorf("round-trip = %q, want hunter2", dec)
	}
}

func TestDecryptPasswordWrongKeyErrors(t *testing.T) {
	enc, err := encryptPassword("hunter2", testHexKey)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	wrongKey := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	if _, err := decryptPassword(enc, wrongKey); err == nil {
		t.Error("expected error decrypting with wrong key")
	}
}

func TestStorePasswordEncryptionFailureNotPersisted(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)
	cfg := &config.Config{EncryptionKey: ""}

	if err := storePassword(settingsRepo, cfg, "app-specific-password"); err == nil {
		t.Fatal("expected error with empty encryption key")
	}
	if got, _ := settingsRepo.Get("imap_password"); got != "" {
		t.Errorf("imap_password should not be written, got %q", got)
	}
}

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
