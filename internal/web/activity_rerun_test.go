package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mojoaar/icloud-mailflow/internal/config"
	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
	"github.com/mojoaar/icloud-mailflow/internal/poller"
)

func TestActivityRerunHandler(t *testing.T) {
	database := openWebTestDB(t)
	rulesRepo := db.NewRulesRepo(database)
	rule := &db.Rule{
		Name: "m", Enabled: true,
		Groups: []db.ConditionGroup{
			{Operator: "AND", Conditions: []db.Condition{
				{Field: "subject", Operator: "contains", Value: "match"},
			}},
		},
		Actions: []db.Action{{Type: "mark_as_read"}},
	}
	if err := rulesRepo.Create(rule); err != nil {
		t.Fatalf("Create: %v", err)
	}

	client := &mockIMAPClient{messages: map[uint32]*imap.Message{5: {UID: 5, Subject: "match me"}}}
	p := poller.NewPoller(client, rulesRepo, nil, nil, db.NewSettingsRepo(database), db.NewStatsRepo(database), db.NewFoldersRepo(database), &config.Config{}, 10, 60, "INBOX", "", nil)

	h := activityRerunHandler(p)
	form := url.Values{"uid": {"5"}, "folder": {"INBOX"}}
	req := httptest.NewRequest("POST", "/activity/rerun", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := serveHandler(h, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "Matched") {
		t.Errorf("expected a matched result, got %q", body)
	}
}

func TestActivityRerunNilPoller(t *testing.T) {
	h := activityRerunHandler(nil)
	req := httptest.NewRequest("POST", "/activity/rerun", strings.NewReader("uid=1&folder=INBOX"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := serveHandler(h, req)
	if !strings.Contains(rec.Body.String(), "IMAP not configured") {
		t.Errorf("expected IMAP not configured toast, got %q", rec.Body.String())
	}
}
