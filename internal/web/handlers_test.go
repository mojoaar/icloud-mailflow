package web

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	goimap "github.com/emersion/go-imap/v2"

	"github.com/mojoaar/icloud-mailflow/internal/config"
	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
	"github.com/mojoaar/icloud-mailflow/internal/poller"
)

func TestSettingsSaveTimezone(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)

	h := settingsSaveTimezone(settingsRepo)
	form := url.Values{"timezone": {"Europe/Copenhagen"}}
	req := httptest.NewRequest("POST", "/settings/timezone", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := serveHandler(h, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
	if v, _ := settingsRepo.Get("timezone"); v != "Europe/Copenhagen" {
		t.Errorf("timezone = %q, want Europe/Copenhagen", v)
	}
}

func TestSettingsSaveFontMonospace(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)

	h := settingsSaveFont(settingsRepo)
	form := url.Values{"font": {"mono"}}
	req := httptest.NewRequest("POST", "/settings/font", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := serveHandler(h, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
	if v, _ := settingsRepo.Get("font_mono"); v != "true" {
		t.Errorf("font_mono = %q, want true", v)
	}
}

func TestSettingsSaveFontDefault(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)
	settingsRepo.Set("font_mono", "true")

	h := settingsSaveFont(settingsRepo)
	form := url.Values{"font": {"default"}}
	req := httptest.NewRequest("POST", "/settings/font", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := serveHandler(h, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
	if v, _ := settingsRepo.Get("font_mono"); v != "false" {
		t.Errorf("font_mono = %q, want false", v)
	}
}

func TestSettingsSavePoll(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)
	cfg := &config.Config{}

	h := settingsSavePoll(cfg, settingsRepo)
	form := url.Values{
		"source_folder": {"Archive"},
		"poll_interval": {"120"},
		"poll_batch":    {"10"},
		"log_keep":      {"500"},
	}
	req := httptest.NewRequest("POST", "/settings/poll", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := serveHandler(h, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
	if cfg.SourceFolder != "Archive" {
		t.Errorf("cfg.SourceFolder = %q, want Archive", cfg.SourceFolder)
	}
	if cfg.PollInterval != 120 {
		t.Errorf("cfg.PollInterval = %d, want 120", cfg.PollInterval)
	}
	if v, _ := settingsRepo.Get("poll_interval"); v != "120" {
		t.Errorf("poll_interval = %q, want 120", v)
	}
}

func TestSettingsSaveBackup(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)

	h := settingsSaveBackup(settingsRepo)
	form := url.Values{
		"backup_enabled":   {"true"},
		"backup_frequency": {"weekly"},
		"backup_recipient": {"backup@example.com"},
	}
	req := httptest.NewRequest("POST", "/settings/backup/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := serveHandler(h, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
	if v, _ := settingsRepo.Get("backup_enabled"); v != "true" {
		t.Errorf("backup_enabled = %q, want true", v)
	}
	if v, _ := settingsRepo.Get("backup_frequency"); v != "weekly" {
		t.Errorf("backup_frequency = %q, want weekly", v)
	}
}

func TestSettingsSaveWebhook(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)

	h := settingsSaveWebhook(settingsRepo)
	form := url.Values{"webhook_secret": {"s3cret"}}
	req := httptest.NewRequest("POST", "/settings/webhook", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := serveHandler(h, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
	if v, _ := settingsRepo.Get("webhook_secret"); v != "s3cret" {
		t.Errorf("webhook_secret = %q, want s3cret", v)
	}
}

func TestSettingsMcpToggle(t *testing.T) {
	t.Run("enable", func(t *testing.T) {
		database := openWebTestDB(t)
		settingsRepo := db.NewSettingsRepo(database)

		h := settingsMcpToggle(settingsRepo)
		req := httptest.NewRequest("POST", "/settings/mcp/toggle", nil)
		rec := serveHandler(h, req)

		if rec.Code != http.StatusSeeOther {
			t.Errorf("status = %d, want 303", rec.Code)
		}
		if v, _ := settingsRepo.Get("mcp_enabled"); v != "true" {
			t.Errorf("mcp_enabled = %q, want true", v)
		}
		key, _ := settingsRepo.Get("mcp_api_key")
		if len(key) == 0 {
			t.Error("mcp_api_key not generated")
		}
	})

	t.Run("disable", func(t *testing.T) {
		database := openWebTestDB(t)
		settingsRepo := db.NewSettingsRepo(database)
		settingsRepo.Set("mcp_enabled", "true")
		settingsRepo.Set("mcp_api_key", "old-key")

		h := settingsMcpToggle(settingsRepo)
		req := httptest.NewRequest("POST", "/settings/mcp/toggle", nil)
		rec := serveHandler(h, req)

		if rec.Code != http.StatusSeeOther {
			t.Errorf("status = %d, want 303", rec.Code)
		}
		if v, _ := settingsRepo.Get("mcp_enabled"); v != "false" {
			t.Errorf("mcp_enabled = %q, want false", v)
		}
	})
}

func TestSettingsMcpRegenerate(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)
	settingsRepo.Set("mcp_api_key", "old-key")

	h := settingsMcpRegenerate(settingsRepo)
	req := httptest.NewRequest("POST", "/settings/mcp/regenerate", nil)
	rec := serveHandler(h, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
	key, _ := settingsRepo.Get("mcp_api_key")
	if key == "old-key" {
		t.Error("mcp_api_key was not regenerated")
	}
	if len(key) != 64 {
		t.Errorf("mcp_api_key hex length = %d, want 64", len(key))
	}
}

func TestSettingsContactsToggle(t *testing.T) {
	t.Run("enable", func(t *testing.T) {
		database := openWebTestDB(t)
		settingsRepo := db.NewSettingsRepo(database)
		settingsRepo.Set("contacts_collection_enabled", "false")

		h := settingsContactsToggle(settingsRepo)
		req := httptest.NewRequest("POST", "/settings/contacts/toggle", nil)
		rec := serveHandler(h, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
		if v, _ := settingsRepo.Get("contacts_collection_enabled"); v != "true" {
			t.Errorf("contacts_collection_enabled = %q, want true", v)
		}
	})

	t.Run("disable", func(t *testing.T) {
		database := openWebTestDB(t)
		settingsRepo := db.NewSettingsRepo(database)
		settingsRepo.Set("contacts_collection_enabled", "true")

		h := settingsContactsToggle(settingsRepo)
		req := httptest.NewRequest("POST", "/settings/contacts/toggle", nil)
		rec := serveHandler(h, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
		if v, _ := settingsRepo.Get("contacts_collection_enabled"); v != "false" {
			t.Errorf("contacts_collection_enabled = %q, want false", v)
		}
	})
}

func TestSettingsContactsWipe(t *testing.T) {
	database := openWebTestDB(t)
	contactsRepo := db.NewContactsRepo(database)
	contactsRepo.Upsert("test@example.com", "")

	h := settingsContactsWipe(contactsRepo)
	req := httptest.NewRequest("POST", "/settings/contacts/wipe", nil)
	rec := serveHandler(h, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "All contacts wiped") {
		t.Errorf("expected 'All contacts wiped', got %q", body)
	}
	results, _ := contactsRepo.Search("test@example.com")
	if len(results) != 0 {
		t.Errorf("expected 0 contacts, got %d", len(results))
	}
}

func testPoller(database *sql.DB, mockIMAP imap.Client) *poller.Poller {
	return poller.NewPoller(
		mockIMAP,
		db.NewRulesRepo(database),
		nil,
		db.NewLogRepo(database),
		db.NewSettingsRepo(database),
		db.NewStatsRepo(database),
		db.NewFoldersRepo(database),
		&config.Config{},
		10, 120, "Processing", "me@icloud.com",
		nil,
	)
}

func TestPollerTickHandlerSuccess(t *testing.T) {
	database := openWebTestDB(t)
	mockIMAP := &mockIMAPClient{searchUIDs: []goimap.UID{}}
	p := testPoller(database, mockIMAP)

	h := pollerTickHandler(p)
	req := httptest.NewRequest("POST", "/poller/tick", nil)
	rec := serveHandler(h, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Poll complete") {
		t.Errorf("expected 'Poll complete', got %q", body)
	}
}

type errorMockIMAP struct {
	mockIMAPClient
}

func (m *errorMockIMAP) SearchMessages(folder string, limit int, minUID uint32) ([]goimap.UID, error) {
	return nil, errors.New("connection refused")
}

func TestPollerTickHandlerError(t *testing.T) {
	database := openWebTestDB(t)
	mockIMAP := &errorMockIMAP{mockIMAPClient: mockIMAPClient{searchUIDs: []goimap.UID{}}}
	p := testPoller(database, mockIMAP)

	h := pollerTickHandler(p)
	req := httptest.NewRequest("POST", "/poller/tick", nil)
	rec := serveHandler(h, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Poll failed") {
		t.Errorf("expected 'Poll failed', got %q", body)
	}
}

func TestDashboardStatusHandler(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)
	settingsRepo.Set("imap_email", "me@icloud.com")
	settingsRepo.Set("source_folder", "Processing")
	settingsRepo.Set("poll_interval", "120")
	rulesRepo := db.NewRulesRepo(database)
	foldersRepo := db.NewFoldersRepo(database)
	foldersRepo.Sync([]db.Folder{{Name: "INBOX", Path: "INBOX"}})
	contactsRepo := db.NewContactsRepo(database)
	contactsRepo.Upsert("test@example.com", "")
	cfg := &config.Config{}

	mockIMAP := &mockIMAPClient{searchUIDs: []goimap.UID{}}
	p := testPoller(database, mockIMAP)

	h := dashboardStatusHandler(p, settingsRepo, mockIMAP, rulesRepo, foldersRepo, contactsRepo, cfg)
	req := httptest.NewRequest("GET", "/dashboard/status", nil)
	rec := serveHandler(h, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "me@icloud.com") {
		t.Error("missing IMAP email in status")
	}
	if !strings.Contains(body, "Processing") {
		t.Error("missing source folder in status")
	}
}

func TestDashboardRulesHandler(t *testing.T) {
	database := openWebTestDB(t)
	rulesRepo := db.NewRulesRepo(database)

	h := dashboardRulesHandler(rulesRepo)
	req := httptest.NewRequest("GET", "/dashboard/rules", nil)
	rec := serveHandler(h, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Rules") {
		t.Error("missing Rules section in rendered content")
	}
}

func TestDocsStandaloneHandler(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)

	h := docsStandaloneHandler(settingsRepo)
	req := httptest.NewRequest("GET", "/docs", nil)
	rec := serveHandler(h, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, appVersion) {
		t.Error("missing version in docs")
	}
}

func TestStatsHandler(t *testing.T) {
	database := openWebTestDB(t)
	statsRepo := db.NewStatsRepo(database)

	h := statsHandler(statsRepo)
	req := httptest.NewRequest("GET", "/stats", nil)
	rec := serveHandler(h, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Stats") {
		t.Error("missing 'Stats' in rendered page")
	}
}
