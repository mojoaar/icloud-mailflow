package web

import (
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func TestActivityHandler(t *testing.T) {
	database := openWebTestDB(t)
	logRepo := db.NewLogRepo(database)

	logRepo.Insert(&db.LogEntry{UID: 1, Subject: "Hello", FromAddr: "a@b.com", RuleName: "test", ActionType: "move", ActionValue: "Trash", Status: "success"})
	logRepo.Insert(&db.LogEntry{UID: 2, Subject: "World", FromAddr: "c@d.com", RuleName: "spam", ActionType: "mark_as_read", ActionValue: "", Status: "error"})

	settingsRepo := db.NewSettingsRepo(database)
	settingsRepo.Set("timezone", "UTC")
	rulesRepo := db.NewRulesRepo(database)

	req := httptest.NewRequest("GET", "/activity", nil)
	rec := httptest.NewRecorder()
	activityHandler(logRepo, rulesRepo, settingsRepo).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Hello") {
		t.Error("body should contain Hello")
	}
	if !strings.Contains(body, "World") {
		t.Error("body should contain World")
	}
}

func TestActivityHandlerEmpty(t *testing.T) {
	database := openWebTestDB(t)
	logRepo := db.NewLogRepo(database)
	settingsRepo := db.NewSettingsRepo(database)
	rulesRepo := db.NewRulesRepo(database)

	req := httptest.NewRequest("GET", "/activity", nil)
	rec := httptest.NewRecorder()
	activityHandler(logRepo, rulesRepo, settingsRepo).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestActivityDeleteHandler(t *testing.T) {
	database := openWebTestDB(t)
	logRepo := db.NewLogRepo(database)

	logRepo.Insert(&db.LogEntry{UID: 1, Subject: "test"})
	logRepo.Insert(&db.LogEntry{UID: 2, Subject: "test"})

	req := httptest.NewRequest("POST", "/activity/delete", nil)
	rec := httptest.NewRecorder()
	activityDeleteHandler(logRepo).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	entries, _ := logRepo.ListRecent(10)
	if len(entries) != 0 {
		t.Errorf("expected empty log after delete, got %d entries", len(entries))
	}
}

func TestActivityDeleteSelectedHandler(t *testing.T) {
	database := openWebTestDB(t)
	logRepo := db.NewLogRepo(database)

	logRepo.Insert(&db.LogEntry{UID: 1, Subject: "a"})
	logRepo.Insert(&db.LogEntry{UID: 2, Subject: "b"})
	logRepo.Insert(&db.LogEntry{UID: 3, Subject: "c"})
	entries, _ := logRepo.ListRecent(10)

	form := url.Values{}
	form.Add("id", strconv.FormatInt(entries[0].ID, 10))
	form.Add("id", strconv.FormatInt(entries[2].ID, 10))
	req := httptest.NewRequest("POST", "/activity/delete-selected", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	activityDeleteSelectedHandler(logRepo).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("HX-Refresh") != "true" {
		t.Error("expected HX-Refresh header")
	}
	left, _ := logRepo.ListRecent(10)
	if len(left) != 1 || left[0].ID != entries[1].ID {
		t.Errorf("remaining = %+v, want only id %d", left, entries[1].ID)
	}
}

func TestActivityDeleteSelectedHandlerNoneSelected(t *testing.T) {
	database := openWebTestDB(t)
	logRepo := db.NewLogRepo(database)
	logRepo.Insert(&db.LogEntry{UID: 1, Subject: "a"})

	req := httptest.NewRequest("POST", "/activity/delete-selected", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	activityDeleteSelectedHandler(logRepo).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("HX-Refresh") == "true" {
		t.Error("must not refresh when nothing is selected")
	}
	left, _ := logRepo.ListRecent(10)
	if len(left) != 1 {
		t.Errorf("expected log untouched, got %d entries", len(left))
	}
}
