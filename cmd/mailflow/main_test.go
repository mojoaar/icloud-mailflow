package main

import (
	"encoding/hex"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mojoaar/icloud-mailflow/internal/config"
	"github.com/mojoaar/icloud-mailflow/internal/crypto"
	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func TestInitialize(t *testing.T) {
	dir := t.TempDir()
	app, err := initialize(dir)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	defer app.Close()

	if app.Config == nil {
		t.Fatal("expected config, got nil")
	}
	if app.Config.IMAPServer != "imap.mail.me.com" {
		t.Fatalf("expected default IMAP server, got %s", app.Config.IMAPServer)
	}
}

func TestInitializeRouterResponds(t *testing.T) {
	dir := t.TempDir()
	app, err := initialize(dir)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	defer app.Close()

	req := httptest.NewRequest("GET", "/login", nil)
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestInitializeSetupPage(t *testing.T) {
	dir := t.TempDir()
	app, err := initialize(dir)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	defer app.Close()

	req := httptest.NewRequest("GET", "/setup", nil)
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAppCloseNoPanic(t *testing.T) {
	dir := t.TempDir()
	app, err := initialize(dir)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	app.Close()
}

func TestAppCloseNilPoller(t *testing.T) {
	app := &App{}
	app.Close()
}

func TestMigrateLegacyIMAPPassword(t *testing.T) {
	dir := t.TempDir()
	key := strings.Repeat("ab", 32)
	raw := `{"imap_server":"imap.mail.me.com","imap_port":993,"imap_email":"u@example.com","source_folder":"Processing","poll_interval":300,"encryption_key":"` + key + `","imap_password":"legacy-secret"}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(raw), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	database, err := db.Open(filepath.Join(dir, "mailflow.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()
	if err := db.Migrate(database); err != nil {
		t.Fatalf("db.Migrate: %v", err)
	}
	settings := db.NewSettingsRepo(database)

	if err := migrateLegacyIMAPPassword(cfg, settings); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	enc, _ := settings.Get("imap_password")
	if enc == "" || enc == "legacy-secret" {
		t.Fatalf("expected an encrypted password in the DB, got %q", enc)
	}
	keyBytes, _ := hex.DecodeString(key)
	dec, err := crypto.Decrypt([]byte(enc), keyBytes)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(dec) != "legacy-secret" {
		t.Errorf("decrypted = %q, want legacy-secret", string(dec))
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(data), "legacy-secret") {
		t.Error("config.json still contains the plaintext password")
	}
}
