package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func TestHealthPublicMinimal(t *testing.T) {
	database := openWebTestDB(t)
	statsRepo := db.NewStatsRepo(database)
	contactsRepo := db.NewContactsRepo(database)
	rulesRepo := db.NewRulesRepo(database)
	sessRepo := db.NewSessionsRepo(database)
	appVersion = "0.8.0"
	startTime = time.Now()

	h := healthHandler(database, nil, nil, statsRepo, contactsRepo, rulesRepo, sessRepo)
	req := httptest.NewRequest("GET", "/health", nil)
	rec := serveHandler(h, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := body["status"]; !ok {
		t.Error("response must include status")
	}
	for _, leaked := range []string{"version", "uptime_seconds", "stats", "poller", "db", "imap"} {
		if _, ok := body[leaked]; ok {
			t.Errorf("unauthenticated /health must not expose %q", leaked)
		}
	}
}

func TestHealthAuthenticatedFull(t *testing.T) {
	database := openWebTestDB(t)
	statsRepo := db.NewStatsRepo(database)
	contactsRepo := db.NewContactsRepo(database)
	rulesRepo := db.NewRulesRepo(database)
	sessRepo := db.NewSessionsRepo(database)
	appVersion = "0.8.0"
	startTime = time.Now()

	token, _ := generateToken()
	if err := sessRepo.Create(token, time.Hour); err != nil {
		t.Fatalf("create session: %v", err)
	}

	h := healthHandler(database, nil, nil, statsRepo, contactsRepo, rulesRepo, sessRepo)
	req := httptest.NewRequest("GET", "/health", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	rec := serveHandler(h, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, want := range []string{"status", "version", "stats", "db", "imap"} {
		if _, ok := body[want]; !ok {
			t.Errorf("authenticated /health must include %q", want)
		}
	}
	if body["version"] != "0.8.0" {
		t.Errorf("version = %v, want 0.8.0", body["version"])
	}
}
